package main

import (
	"bufio"
	"fmt"
	"os"
)

func GetMcServerLogs(filename string) ([]string, error) {
	content, err := ReadLines(filename)
	if err != nil {
		fmt.Println("log errors")
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
