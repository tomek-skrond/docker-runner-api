package main

import (
	"archive/zip"
	"io"
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
