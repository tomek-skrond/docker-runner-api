package main

import (
	"fmt"
	"io/ioutil"
)

func GetMcServerLogs(filename string) (string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Println("log errors")
		return "error reading file, go back to home page", err
	}
	return string(content), err
}
