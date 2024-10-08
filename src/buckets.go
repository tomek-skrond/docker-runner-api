package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	storage "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type Bucket struct {
	Name      string
	projectID string
	isPrivate bool
}

func NewBucket(bucketName string, projectID string) (*Bucket, error) {
	return &Bucket{
		Name:      bucketName,
		projectID: projectID,
		isPrivate: true,
	}, nil
}

// ////////////////////////////////////////////////////////////////////////////////////////////////////////////
// uploadFile uploads an object.
func (b *Bucket) UploadFileToGCS(filePath string) error {
	bucketName := b.Name
	objectName := filepath.Base(filePath)

	// Create a new context
	ctx := context.Background()

	// Create a client
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	defer client.Close()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Create a writer to the bucket and object
	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	defer wc.Close()

	// Copy the file content to GCS
	if _, err = io.Copy(wc, file); err != nil {
		return fmt.Errorf("failed to copy file to GCS: %v", err)
	}

	// Close the writer and check for any errors
	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %v", err)
	}

	log.Printf("File %v uploaded to bucket %v as %v", filePath, bucketName, objectName)
	return nil
}

func (b *Bucket) CreateGCSBucket() error {
	// Setup context and client
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create client")
	}

	// Setup client bucket to work from
	bucket := client.Bucket(b.Name)

	buckets := client.Buckets(ctx, b.projectID)
	for {
		if b.Name == "" {
			return fmt.Errorf("BucketName entered is empty %v.", b.Name)
		}
		attrs, err := buckets.Next()
		// Assume bucket not found if at Iterator end and create
		if err == iterator.Done {
			// Create bucket without public access
			if err := bucket.Create(ctx, b.projectID, &storage.BucketAttrs{
				Location: "US",
				UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
					Enabled: true, // Enforces access control uniformly
				},
			}); err != nil {
				return fmt.Errorf("Failed to create bucket: %v", err)
			}

			log.Printf("Bucket %v created.\n", b.Name)
			return nil
		}
		if err != nil {
			return fmt.Errorf("Issues setting up Bucket(%q).Objects(): %v. Double check project id.", attrs.Name, err)
		}
		if attrs.Name == b.Name {
			log.Printf("Bucket %v exists.\n", b.Name)
			return nil
		}
	}
}

func (b *Bucket) ObjectExists(objectPath string) (bool, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to create GCS client: %v", err)
	}
	defer client.Close()

	// Check if the object exists
	_, err = client.Bucket(b.Name).Object(objectPath).Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (b *Bucket) RetrieveObjectsInBucket(ctx context.Context) ([]string, error) {
	log.Println("creating client")
	client, err := storage.NewClient(ctx)
	if err != nil {
		return []string{}, err
	}
	defer client.Close()

	objects := []string{}

	log.Println("checking if bucket exists")
	if b.BucketExists(ctx, client) {

		bucketName := b.Name
		bucket := client.Bucket(bucketName)
		query := &storage.Query{}

		it := bucket.Objects(ctx, query)
		for {
			objAttrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Fatalf("error listing objects %v\n", err)
				//return nil error to mitigate crash if bucket does not exist
				return []string{}, nil
			}
			objects = append(objects, objAttrs.Name)
		}
	}

	return objects, nil
}

func (b *Bucket) DownloadDataFromBucket(ctx context.Context, objectName string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalln(err)
		return err
	}
	defer client.Close()

	bucketName := b.Name
	localBackupsPath := "backups"

	file, err := os.Create(fmt.Sprintf("backups/%s", objectName))
	if err != nil {
		log.Fatalln("failed to create file", err)
		return err
	}
	defer file.Close()

	bucket := client.Bucket(bucketName)
	object := bucket.Object(objectName)

	reader, err := object.NewReader(ctx)
	if err != nil {
		log.Fatalln("failed to create object reader", err)
		return err
	}
	defer reader.Close()

	if _, err := io.Copy(file, reader); err != nil {
		log.Fatalf("Failed to copy object content to file: %v\n", err)
		return err
	}

	fmt.Printf("Object %s downloaded to %s\n", objectName, localBackupsPath)

	return nil
}

// bucketExists checks if a bucket exists.
func (b *Bucket) BucketExists(ctx context.Context, client *storage.Client) bool {
	bucketName := b.Name
	bucket := client.Bucket(bucketName)
	_, err := bucket.Attrs(ctx)
	if err != nil {
		if storage.ErrBucketNotExist == err {
			return false
		}
		log.Printf("Error checking bucket existence: %v", err)
		return false
	}
	return true
}
