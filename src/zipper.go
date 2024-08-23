package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type BackupTemplateData struct {
	Backups []string
}

func GetAvailableBackups(backupPath string) ([]string, error) {

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
