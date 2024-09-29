package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	storage "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type Bucket struct {
	Name             string `json:"name"`
	ProjectID        string `json:"project_id"`
	localBackupsPath string
	isPrivate        bool
}

func NewBucket(bucketName string, projectID, localBackupsPath string) (*Bucket, error) {
	return &Bucket{
		Name:             bucketName,
		ProjectID:        projectID,
		isPrivate:        true,
		localBackupsPath: localBackupsPath,
	}, nil
}

type BackupService struct {
	bucket     *Bucket
	backupPath string
}

func NewBackupService(bucket *Bucket, backupPath string) *BackupService {
	return &BackupService{
		bucket:     bucket,
		backupPath: backupPath,
	}
}

type BackupData struct {
	BucketData            Bucket        `json:"bucket_data"`
	FileName              string        `json:"file_name"`
	Size                  float64       `json:"size"`
	UploadTime            time.Duration `json:"upload_duration"`
	UploadTimeInSeconds   float64       `json:"upload_time_seconds"`
	DownloadTime          time.Duration `json:"download_duration"`
	DownloadTimeInSeconds float64       `json:"download_duration_seconds"`
	DateAccessed          time.Time     `json:"date_accessed"`
}

func NewBackupData(bucket *Bucket, fileName string, size float64, upload, download *time.Duration, dateAccessed *time.Time) *BackupData {
	if bucket == nil {
		bucket = &Bucket{}
	}
	return &BackupData{
		BucketData:            *bucket,
		FileName:              fileName,
		Size:                  size,
		UploadTime:            *upload,
		UploadTimeInSeconds:   upload.Seconds(),
		DownloadTime:          *download,
		DownloadTimeInSeconds: download.Seconds(),
		DateAccessed:          *dateAccessed,
	}
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

	buckets := client.Buckets(ctx, b.ProjectID)
	for {
		if b.Name == "" {
			return fmt.Errorf("BucketName entered is empty %v.", b.Name)
		}
		attrs, err := buckets.Next()
		// Assume bucket not found if at Iterator end and create
		if err == iterator.Done {
			// Create bucket without public access
			if err := bucket.Create(ctx, b.ProjectID, &storage.BucketAttrs{
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
				log.Printf("error listing objects %v\n", err)
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
		log.Println(err)
		return err
	}
	defer client.Close()

	bucketName := b.Name
	localBackupsPath := b.localBackupsPath

	file, err := os.Create(fmt.Sprintf("%s/%s", b.localBackupsPath, objectName))
	if err != nil {
		log.Println("failed to create file", err)
		return err
	}
	defer file.Close()

	bucket := client.Bucket(bucketName)
	object := bucket.Object(objectName)

	reader, err := object.NewReader(ctx)
	if err != nil {
		log.Println("failed to create object reader", err)
		return err
	}
	defer reader.Close()

	if _, err := io.Copy(file, reader); err != nil {
		log.Printf("Failed to copy object content to file: %v\n", err)
		return err
	}

	log.Printf("Object %s downloaded to %s\n", objectName, localBackupsPath)

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

func (bs *BackupService) Sync() (*[]BackupData, error) {
	if bs.bucket.ProjectID == "" || bs.bucket.Name == "" {
		return nil, errors.New("synchronization bucket information not complete")
	}

	backupsOnDisk, err := GetAvailableLocalBackups(fmt.Sprintf("%s/", bs.backupPath))
	if err != nil {
		return nil, err
	}

	if err := bs.bucket.CreateGCSBucket(); err != nil {
		return nil, err
	}

	backupsOnCloud, err := bs.bucket.RetrieveObjectsInBucket(context.Background())
	if err != nil {
		return nil, err
	}

	backupsMissingOnCloud := stringArrayDiff(backupsOnCloud, backupsOnDisk)
	backupsMissingOnDisk := stringArrayDiff(backupsOnDisk, backupsOnCloud)

	var backupDataList []BackupData

	log.Println("backups missing on the cloud: ", backupsMissingOnCloud)
	log.Println("backups missing on the disk: ", backupsMissingOnDisk)

	for _, fileName := range backupsMissingOnDisk {
		timeFromCloud, sizeFromCloud, err := bs.DownloadFileFromCloud(fileName)
		if err != nil {
			return nil, err
		}

		downloadDuration := timeFromCloud
		uploadDuration := time.Duration(float64(0))

		now := time.Now()
		backupData := NewBackupData(bs.bucket, fileName, float64(sizeFromCloud), &uploadDuration, &downloadDuration, &now)
		backupDataList = append(backupDataList, *backupData)
	}

	for _, fileName := range backupsMissingOnCloud {
		timeToCloud, sizeToCloud, err := bs.UploadFileToCloud(fileName)
		if err != nil {
			return nil, err
		}
		uploadDuration := timeToCloud
		downloadDuration := time.Duration(float64(0))

		now := time.Now()
		backupData := NewBackupData(bs.bucket, fileName, float64(sizeToCloud), &uploadDuration, &downloadDuration, &now)
		backupDataList = append(backupDataList, *backupData)
	}

	return &backupDataList, nil
}

func (bs *BackupService) UploadDataToCloud(backupsStrArr []string) error {
	for _, backup := range backupsStrArr {
		objectPath := fmt.Sprintf("%s/%s", bs.backupPath, backup)

		// Check if the object already exists in GCS
		log.Println("check if object exists", backup)
		exists, err := bs.bucket.ObjectExists(backup)
		if err != nil {
			log.Printf("Error checking if object exists in GCS: %v", err)
			return err
		}
		if exists {
			log.Printf("Object %s already exists in GCS. Skipping upload.", objectPath)
			continue
		}
		log.Printf("uploading file %s to GCS\n", backup)
		if err := bs.bucket.UploadFileToGCS(objectPath); err != nil {
			log.Println(err)
			return err
		}
	}
	log.Println("uploading data to cloud successful")

	return nil
}

func (bs *BackupService) UploadFileToCloud(backupName string) (time.Duration, int64, error) {
	start := time.Now()

	objectPath := fmt.Sprintf("%s/%s", bs.backupPath, backupName)

	// Check if the object already exists in GCS
	log.Println("check if object exists", backupName)
	exists, err := bs.bucket.ObjectExists(backupName)
	if err != nil {
		log.Printf("Error checking if object exists in GCS: %v", err)
		return 0, 0, err
	}
	if exists {
		log.Printf("Object %s already exists in GCS. Skipping upload.", objectPath)
		return 0, 0, nil
	}

	filePath := fmt.Sprintf("%s/%s", bs.backupPath, backupName)
	fileSize, err := getFileSize(filePath)
	if err != nil {
		return 0, 0, err
	}

	log.Printf("uploading file %s to GCS\n", backupName)
	if err := bs.bucket.UploadFileToGCS(objectPath); err != nil {
		log.Println(err)
		return 0, 0, err
	}

	duration := time.Since(start)
	if exists {
		duration = 0
	}
	log.Println("uploading data to cloud successful")
	return duration, fileSize, nil
}

func (bs *BackupService) DownloadDataFromCloud(backupsInCloud []string) error {
	log.Println("getting available backups from disk")
	backupsOnDisk, err := GetAvailableLocalBackups(bs.backupPath)
	if err != nil {
		log.Println(err)
		return err
	}
	for _, backup := range backupsInCloud {
		if !contains(backupsOnDisk, backup) {
			log.Printf("downloading backup %s from cloud", backup)
			if err := bs.bucket.DownloadDataFromBucket(context.Background(), backup); err != nil {
				log.Println(err)
				return err
			}
		}
	}
	log.Println("downloading data to disk successful")

	return nil
}

func (bs *BackupService) DownloadFileFromCloud(backup string) (time.Duration, int64, error) {
	start := time.Now()

	log.Println("getting available backups from disk")
	backupsOnDisk, err := GetAvailableLocalBackups(bs.backupPath + "/")
	if err != nil {
		log.Println(err)
		return 0, 0, err
	}

	var exists bool
	if !contains(backupsOnDisk, backup) {
		log.Printf("downloading backup %s from cloud", backup)
		if err := bs.bucket.DownloadDataFromBucket(context.Background(), backup); err != nil {
			log.Println(err)
			return 0, 0, err
		}
		exists = false
	} else {
		exists = true
	}

	duration := time.Since(start)
	filePath := fmt.Sprintf("%s/%s", bs.backupPath, backup)
	fileSize, err := getFileSize(filePath)
	if err != nil {
		return duration, 0, err
	}
	if exists {
		duration = 0
	}
	log.Println("downloading data to disk successful")
	return duration, fileSize, nil
}

func (bs *BackupService) UploadBackupMultipart(progressReader *ProgressReader, fileName string) (*BackupData, error) {
	// Create the "backups/" directory if it doesn't exist
	backupDir := bs.backupPath
	fmt.Println(backupDir)
	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		return nil, err
	}

	// Create a temporary file in the backups directory to store the uploaded data
	tempFilePath := fmt.Sprintf("%s/%s", backupDir, fileName)
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()

	// Write the uploaded file data to the temp file, logging progress
	if _, err := io.Copy(tempFile, progressReader); err != nil {
		return nil, err
	}

	// Call your method to handle the file
	data, err := bs.LoadBackupChooseFile(tempFile, tempFilePath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (bs *BackupService) LoadBackupChooseFile(file multipart.File, backupPath string) (*BackupData, error) {

	backupDir := bs.backupPath
	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		return nil, err
	}

	// Create the destination file path in the backups directory
	destFilePath := backupPath

	// Open the destination file for writing
	destFile, err := os.Create(destFilePath)
	if err != nil {
		return nil, err
	}
	defer destFile.Close()

	// Copy the file from the temporary location to the backups directory
	start := time.Now()
	writtenSize, err := io.Copy(destFile, file)
	if err != nil {
		return nil, err
	}

	// Calculate upload time
	uploadTime := time.Since(start)
	downloadTime := time.Duration(0)
	now := time.Now()

	// Create the BackupData object
	data := NewBackupData(nil, backupPath, float64(writtenSize), &uploadTime, &downloadTime, &now)

	return data, nil
}

func (bs *BackupService) LoadBackupFromDisk(backupFile string) (*BackupData, error) {

	backups := bs.backupPath
	log.Println("loading new backup initiated")
	currentTime := time.Now()

	formattedTime := currentTime.Format("20060102_150405")

	fileName := fmt.Sprintf("%s_%s.zip", "mcdata", formattedTime)

	if err := zipit("mcdata", backups+"/"+fileName, false); err != nil {
		log.Println(err)
		return nil, err
	}

	if err := removeAllFilesInDir("mcdata"); err != nil {
		log.Println(err)
		return nil, err
	}

	if err := unzip(fmt.Sprintf("%s/%s", backups, backupFile), "mcdata"); err != nil {
		log.Println(err)
		return nil, err
	}

	downloadTime := time.Duration(0)
	uploadTime := time.Since(currentTime)
	now := time.Now()
	fileSize, err := getFileSize(fmt.Sprintf("%s/%s", backups, fileName))
	if err != nil {
		return nil, err
	}
	data := NewBackupData(nil, fileName, float64(fileSize), &uploadTime, &downloadTime, &now)
	return data, err
}

func (bs *BackupService) GetBackups() ([]string, error) {
	backupPath := bs.backupPath
	backups, err := GetAvailableLocalBackups(backupPath)
	if err != nil {
		return nil, err
	}
	return backups, nil
}

func (bs *BackupService) Backup(backupName string) (*BackupData, error) {
	// Perform backup operation here
	currentTime := time.Now()
	formattedTime := currentTime.Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s.zip", backupName, formattedTime)

	if err := zipit("mcdata", bs.backupPath+"/"+fileName, false); err != nil {
		return nil, err
	}
	duration := time.Since(currentTime)
	size, err := getFileSize(fmt.Sprintf("%s/%s", bs.backupPath, fileName))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	downloadDuration := time.Duration(0)
	data := NewBackupData(nil, backupName, float64(size), &duration, &downloadDuration, &now)
	return data, nil
}
