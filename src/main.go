package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func main() {
	bindPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	img := "itzg/minecraft-server"
	bm := true
	cn := "bebok"
	ports, err := nat.NewPort("tcp", "25565-25565")
	if err != nil {
		panic(err)
	}
	conf := container.Config{
		Hostname:     "minecraft",
		Image:        img,
		ExposedPorts: nat.PortSet{ports: struct{}{}},
		Env:          []string{"EULA=TRUE"},
	}
	hostconf := container.HostConfig{
		Resources: container.Resources{
			Memory: 4 * 2147483648,
		},
		Binds: []string{
			fmt.Sprintf("%v/mcdata:/data", bindPath),
		},
		PortBindings: nat.PortMap{
			"25565/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "9999",
				},
			},
		},
		AutoRemove: true,
	}
	netconf := network.NetworkingConfig{}
	platform := v1.Platform{}
	pullopts := types.ImagePullOptions{}
	startopts := types.ContainerStartOptions{}

	if hostconf.AutoRemove != true {
		hostconf.AutoRemove = true
	}

	runner := NewContainerRunner(
		img,
		bm,
		cn,
		conf,
		hostconf,
		netconf,
		platform,
		pullopts,
		startopts)

	listenPort := ":7777"

	server := NewAPIServer(listenPort, runner)

	server.Run()
}
