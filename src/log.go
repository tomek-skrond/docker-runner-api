package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

func LogServer() {
	filename := ""
	names := make(chan string)
	readerr := make(chan error)
	done := make(chan bool)

	go GetLine(filename, names, readerr, done)

loop:
	for {
		select {
		case name := <-names:
			// Process each line
			fmt.Println(name)

		case err := <-readerr:
			if err != nil {
				log.Fatal(err)
			}
			break loop
		}
	}
	fmt.Println("processing complete")
}

func GetLine(filename string, names chan string, readerr chan error, done chan bool) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("error reading file")
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		names <- scanner.Text()
		fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		readerr <- err
	}

	done <- true
}

func GetMcServerLogs(filename string) string {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("log errors")
		panic(err)
	}
	return string(content)
}
