package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type ContainerRunner struct {
	Image         string                      // image for a container
	BuildMode     bool                        // build mode (if set to true -> container builds from Dockerfile)
	ContainerName string                      // container name
	NetworkName   string                      // network name
	Context       context.Context             // context (needed for Docker API functions) from "context" package
	Client        *client.Client              // docker client
	Config        container.Config            // configuration scheme for docker container
	HostConf      container.HostConfig        // docker host config
	NetConf       network.NetworkingConfig    // networking config for container
	Platform      v1.Platform                 // platform
	PullOpts      types.ImagePullOptions      // pull options (applies for BuildMode: false)
	StartOpts     types.ContainerStartOptions // start options
}

func NewContainerRunner(img string,
	cn string,
	netname string,
	conf container.Config,
	hostconf container.HostConfig,
	netconf network.NetworkingConfig,
	platform v1.Platform,
	pullopts types.ImagePullOptions,
	startopts types.ContainerStartOptions) *ContainerRunner {
	return &ContainerRunner{
		Image:         img,
		ContainerName: cn,
		NetworkName:   netname,
		Context:       context.Background(),
		Client:        &client.Client{},
		Config:        conf,
		HostConf:      hostconf,
		NetConf:       netconf,
		Platform:      platform,
		PullOpts:      pullopts,  //types.ImagePullOptions{},
		StartOpts:     startopts, //&types.ContainerStartOptions{},
	}
}

func (r *ContainerRunner) InitializeClient() error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Println("Pull error")
		return err
	}
	r.Client = cli
	defer r.Client.Close()

	return nil
}

func (r *ContainerRunner) PullDockerImage() (io.ReadCloser, error) {

	out, err := r.Client.ImagePull(r.Context, r.Image, r.PullOpts)
	if err != nil {
		log.Println("pull errors?")
		panic(err)
	}
	defer out.Close()
	io.Copy(os.Stdout, out)

	return out, nil
}

// example usage
// buildopts := types.ImageBuildOptions{
// 	Tags:           []string{"my-image:latest"},
// 	Dockerfile:     "path/to/Dockerfile",
// 	Remove:         true,
// 	ForceRemove:    true,
// 	SuppressOutput: false,
// }

func ExtractImageID(buildResponse types.ImageBuildResponse) (string, error) {
	// Read the build response as JSON
	buildResponseBytes, err := ioutil.ReadAll(buildResponse.Body)
	if err != nil {
		return "", err
	}

	// Search for the "aux" field in the JSON response
	var buildAux struct {
		ID string `json:"ID"`
	}
	err = json.Unmarshal(buildResponseBytes, &buildAux)
	if err != nil {
		return "", err
	}

	return buildAux.ID, nil
}

func (r *ContainerRunner) CreateNetwork() (types.NetworkCreateResponse, error) {
	return r.Client.NetworkCreate(
		r.Context,
		r.NetworkName,
		types.NetworkCreate{},
	)
}
func (r *ContainerRunner) CreateContainer() (container.CreateResponse, error) {
	return r.Client.ContainerCreate(
		r.Context,
		&r.Config,
		&r.HostConf,
		&r.NetConf,
		&r.Platform,
		r.ContainerName)
}

func (r *ContainerRunner) StartContainer(resp container.CreateResponse) error {
	return r.Client.ContainerStart(r.Context, resp.ID, r.StartOpts)
}

func (r *ContainerRunner) Containerize() {

	if err := r.InitializeClient(); err != nil {
		log.Println("init client error")
		panic(err)
	}
	log.Println("initialized client")

	out, err := r.PullDockerImage()
	if err != nil {
		log.Println(out)
		log.Println("Pull error")
		panic(err)
	}

	netresp, err := r.CreateNetwork()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created network with name %v and ID: %v\n", r.NetworkName, netresp.ID)

	resp, err := r.CreateContainer()
	if err != nil {
		panic(err)
	}

	if err := r.StartContainer(resp); err != nil {
		log.Println("start container error")
		panic(err)
	}
	log.Println("ID of created container: ", resp.ID)

}

func (r *ContainerRunner) StopContainer() {
	if err := r.InitializeClient(); err != nil {
		log.Println("Error initializing client")
		panic(err)
	}

	noWaitTimeout := 0
	containerFilters := filters.NewArgs()
	containerFilters.Add("name", r.ContainerName)

	networkFilters := filters.NewArgs()
	networkFilters.Add("name", r.NetworkName)

	containers, err := r.Client.ContainerList(r.Context, types.ContainerListOptions{Filters: containerFilters})
	networks, err := r.Client.NetworkList(r.Context, types.NetworkListOptions{Filters: networkFilters})

	if err != nil {
		panic(err)
	}

	if len(containers) == 0 && len(networks) == 0 {
		log.Println("Container does not exist")
	}

	if len(containers) == 1 {
		log.Println("container ID found:", containers[0].ID)
		if err := r.Client.ContainerStop(r.Context, r.ContainerName, container.StopOptions{Timeout: &noWaitTimeout}); err != nil {
			panic(err)
		}
		log.Println("Success stopping container ", r.ContainerName)

	}

	if len(networks) == 1 {
		log.Println("network ID found:", networks[0].ID)
		if err := r.Client.NetworkRemove(r.Context, r.NetworkName); err != nil {
			panic(err)
		}
		log.Println("Success removing network ", r.NetworkName)
	}

}
