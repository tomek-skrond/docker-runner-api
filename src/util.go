package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/exp/rand"
)

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyz"
	digitBytes  = "0123456789"
)

func inTrustedRoot(path string, trustedRoot string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return errors.New("error determining absolute path")
	}

	// Ensure the resolved path starts with the trusted root
	if !filepath.HasPrefix(absPath, trustedRoot) {
		return errors.New("path is outside of trusted root")
	}

	return nil
}

// resolveAndCheckPath resolves the symlinks and checks if the parent directory is within the trusted root
func resolveAndCheckPath(trustedPath string, path BindPath) (string, error) {
	// Clean the path
	cleanedPath := filepath.Clean(path.Path)
	fullPath, err := filepath.Abs(cleanedPath)
	if err != nil {
		return fullPath, errors.New("cannot resolve full path")
	}

	// // Resolve symlinks for the parent directory
	// parentDir := filepath.Dir(cleanedPath)
	// resolvedParentDir, err := filepath.EvalSymlinks(parentDir)
	// if err != nil {
	// 	fmt.Println(resolvedParentDir)
	// 	return cleanedPath, errors.New("error resolving symlinks for parent directory")
	// }

	// Verify if the resolved parent directory is within the trusted root
	err = inTrustedRoot(fullPath, trustedPath)
	if err != nil {
		return cleanedPath, errors.New("path is outside of trusted root")
	}

	log.Printf("checking existence of %s path: %s\n", path.Label, fullPath)
	// Check if the directory exists, and create it if not
	err = createDirectoryIfNotExists(fullPath)
	if err != nil {
		return fullPath, err
	}

	if path.Label == "serverfiles" {
		log.Printf("checking existence of minecraft data folder")
		err = createDirectoryIfNotExists(fmt.Sprintf("%s/mcdata", fullPath))
		if err != nil {
			return fullPath, err
		}
	}
	return fullPath, nil
}

// createDirectoryIfNotExists checks if the directory exists, and creates it if not
func createDirectoryIfNotExists(path string) error {
	// Check if the path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Directory does not exist, create it
		err := os.MkdirAll(path, 0755) // 0755 is the permission mode for the directory
		if err != nil {
			return fmt.Errorf("failed to create directory: %s, error: %v", path, err)
		}
		log.Printf("created directory: %s\n", path)
	} else if err != nil {
		return fmt.Errorf("error checking directory: %s, error: %v", path, err)
	}
	return nil
}

// verifyPath checks if the path is valid based on the OS
func verifyPath(trustedPath string, path BindPath) (string, error) {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		log.Println("established trusted path to be: ", trustedPath)
		return resolveAndCheckPath(trustedPath, path)
	}

	if runtime.GOOS == "windows" {
		return path.Path, fmt.Errorf("unimplemented")
	}

	return path.Path, fmt.Errorf("runtime not implemented: %s", runtime.GOOS)
}

func randomString(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		if rand.Intn(2) == 0 { // 50% chance of letter, 50% chance of digit
			sb.WriteByte([]byte(letterBytes)[rand.Intn(len(letterBytes))])
		} else {
			sb.WriteByte([]byte(digitBytes)[rand.Intn(len(digitBytes))])
		}
	}
	return sb.String()
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getFileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}

func stringArrayDiff(arr1, arr2 []string) []string {
	// Create a map to track the elements of arr1
	elements := make(map[string]bool)
	for _, item := range arr1 {
		elements[item] = true
	}

	// Create a slice to hold the missing elements
	var missing []string
	for _, item := range arr2 {
		// If the item from arr2 is not in arr1, add it to missing
		if !elements[item] {
			missing = append(missing, item)
		}
	}

	return missing
}

// messageToJSON constructs the JSON response from the provided parameters
func messageToJSON(status int, msg string, content any) JSONResponse {
	return JSONResponse{
		ResponseContent: content,
		HTTPStatus:      status,
		Message:         msg,
	}
}

// WriteJSON writes the JSON response to the http.ResponseWriter
func WriteJSON(w http.ResponseWriter, content JSONResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(content.HTTPStatus)

	// Convert the content to JSON and write to the response
	if err := json.NewEncoder(w).Encode(content); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
	}
}

// ProgressReader wraps an io.Reader to log the progress of reading data
type ProgressReader struct {
	Reader       io.Reader
	TotalBytes   int64
	LoggedBytes  int64
	Logger       *log.Logger
	NextLogPoint int64
}

// Read overrides the Read method to add progress logging
func (p *ProgressReader) Read(b []byte) (int, error) {
	n, err := p.Reader.Read(b)
	if n > 0 {
		p.LoggedBytes += int64(n)
		percentage := float64(p.LoggedBytes) / float64(p.TotalBytes) * 100

		if p.NextLogPoint == 0 {
			p.NextLogPoint = 5
		}

		if percentage >= float64(p.NextLogPoint) {
			p.Logger.Printf("Uploaded %.0f%%", percentage)
			p.NextLogPoint += 5
		}
	}

	return n, err
}

type BackupTemplateData struct {
	Backups      []string
	CloudBackups []string
}

func GetAvailableLocalBackups(backupPath string) ([]string, error) {

	// Open the directory
	files, err := os.ReadDir(backupPath)
	if err != nil {
		log.Fatalln(err)
	}

	regexPattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+_\d{8}_\d{6}\.(zip|tar\.gz|gz|bz2|7z|xz)$`)

	var filesStrArr []string
	// Loop through the directory and filter files
	for _, file := range files {
		// Check if the file matches the regex and is not a directory
		if !file.IsDir() && regexPattern.MatchString(file.Name()) {
			filesStrArr = append(filesStrArr, file.Name())
		}
	}

	return filesStrArr, nil
}

// src code credits: https://gist.github.com/yhirose/addb8d248825d373095c
func zipit(source, target string, needBaseDir bool) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			if needBaseDir {
				header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
			} else {
				path := strings.TrimPrefix(path, source)
				if len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
					path = path[1:]
				}
				if len(path) == 0 {
					return nil
				}
				header.Name = path
			}
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})

	return err
}

func unzip(archive, target string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		path := filepath.Join(target, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			return err
		}
	}

	return nil
}

func removeAllFilesInDir(dir string) error {
	// Get a list of all files in the directory
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Iterate through the files and remove each one
	for _, file := range files {
		// Construct full file path
		filePath := filepath.Join(dir, file.Name())

		// Check if it's a file (not a directory)
		if !file.IsDir() {
			err := os.Remove(filePath)
			if err != nil {
				return fmt.Errorf("failed to remove file: %w", err)
			}
		}
	}

	return nil
}

func GetMcServerLogs(filename string) ([]string, error) {
	content, err := ReadLines(filename)
	if err != nil {
		log.Println("errors reading logs")
		return []string{"error reading file, go back to home page"}, err
	}
	return content, err
}

func ReadLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
