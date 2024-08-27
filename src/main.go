package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func main() {
	// server params
	listenPort := ":7777"
	backupPath := "backups"

	// container params
	img := "itzg/minecraft-server"
	cn := "bebok"

	// needed envs
	secret := os.Getenv("JWT_SECRET")
	bucketName := os.Getenv("BACKUPS_BUCKET")
	projectID := os.Getenv("PROJECT_ID")

	// create backups directory
	doesExistBackups, _ := exists("backups")
	if !doesExistBackups {
		if err := os.Mkdir("backups", os.FileMode(0755)); err != nil {
			log.Fatalln("cannot create directory", err)
			panic(err)
		}
	}

	doesExistMcData, _ := exists("mcdata")
	if !doesExistMcData {
		if err := os.Mkdir("mcdata", os.FileMode(0755)); err != nil {
			log.Fatalln("cannot create directory", err)
			panic(err)
		}
	}

	//init server
	bindPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	logPath := fmt.Sprintf("%v/mcdata/logs/latest.log", bindPath)

	// create login service
	loginSvc := NewLoginService(secret)

	// create runner
	runner := InitRunner(img, cn, bindPath)

	// create bucket controller
	bucket, err := InitBucket(bucketName, projectID)
	if err != nil {
		log.Fatalln(err)
	}

	backupSvc := NewBackupService(bucket, backupPath)

	// create API server instance
	server := NewAPIServer(listenPort, logPath, loginSvc, runner, backupSvc, secret)
	server.Run()
}

func InitBucket(bucketName, projectID string) (*Bucket, error) {
	bucket, err := NewBucket(bucketName, projectID)
	if err != nil {
		log.Fatalln(err)
		return nil, err
	}

	return bucket, nil
}

func InitRunner(containerImage, containerName, bindPath string) *ContainerRunner {
	img := containerImage
	cn := containerName

	ports, err := nat.NewPort("tcp", "25565-25565")
	if err != nil {
		panic(err)
	}
	networkName := fmt.Sprintf("mcnet-%d", rand.IntN(10000))

	conf := container.Config{
		Hostname:     "minecraft",
		Image:        img,
		ExposedPorts: nat.PortSet{ports: struct{}{}},
		Env:          []string{"EULA=TRUE"},
		User:         fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), // Match the current user
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
					HostPort: "25565",
				},
			},
		},
		AutoRemove:  true,
		NetworkMode: container.NetworkMode(container.NetworkMode(networkName).NetworkName()),
	}
	netconf := network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {
				NetworkID: networkName,
			},
		},
	}
	platform := v1.Platform{}
	pullopts := types.ImagePullOptions{}
	startopts := types.ContainerStartOptions{}

	if !hostconf.AutoRemove {
		hostconf.AutoRemove = true
	}

	runner := NewContainerRunner(
		img,
		cn,
		networkName,
		conf,
		hostconf,
		netconf,
		platform,
		pullopts,
		startopts)

	return runner
}
