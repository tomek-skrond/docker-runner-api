package main

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

func main() {

	runner := NewContainerRunner("nginx", context.Background(), &client.Client{})
	listenPort := ":7777"

	server, err := NewAPIServer(listenPort, runner)
	if err != nil {
		fmt.Println("server error")
		panic(err)
	}

	server.Run()
}
