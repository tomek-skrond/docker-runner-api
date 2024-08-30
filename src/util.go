package main

import (
	"encoding/json"
	"net/http"
	"os"
)

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
