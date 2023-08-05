package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	bm bool,
	cn string,
	conf container.Config,
	hostconf container.HostConfig,
	netconf network.NetworkingConfig,
	platform v1.Platform,
	pullopts types.ImagePullOptions,
	startopts types.ContainerStartOptions) *ContainerRunner {
	return &ContainerRunner{
		Image:         img,
		BuildMode:     bm,
		ContainerName: cn,
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
		fmt.Println("Pull error")
		return err
	}
	r.Client = cli
	defer r.Client.Close()

	return nil
}

func (r *ContainerRunner) PullDockerImage() (io.ReadCloser, error) {

	out, err := r.Client.ImagePull(r.Context, r.Image, r.PullOpts)
	if err != nil {
		fmt.Println("pull errors?")
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
		fmt.Println("init client error")
		panic(err)
	}
	fmt.Println("initialized client")

	out, err := r.PullDockerImage()
	if err != nil {
		fmt.Println(out)
		fmt.Println("Pull error")
		panic(err)
	}

	resp, err := r.CreateContainer()
	if err != nil {
		panic(err)
	}

	if err := r.StartContainer(resp); err != nil {
		fmt.Println("start container error")
		panic(err)
	}
	fmt.Println("ID of created container: ", resp.ID)

}

func (r *ContainerRunner) StopContainer() {
	if err := r.InitializeClient(); err != nil {
		fmt.Println("Error initializing client")
		panic(err)
	}

	containername := r.ContainerName
	noWaitTimeout := 0
	filters := filters.NewArgs()
	filters.Add("name", containername)
	containers, err := r.Client.ContainerList(r.Context, types.ContainerListOptions{Filters: filters})
	if err != nil {
		panic(err)
	}

	if len(containers) == 0 {
		fmt.Println("Container does not exist")
	}

	if len(containers) == 1 {
		fmt.Println("container ID found:", containers[0].ID)

		if err := r.Client.ContainerStop(r.Context, containername, container.StopOptions{Timeout: &noWaitTimeout}); err != nil {
			panic(err)
		}
		fmt.Println("Success stopping container ", r.ContainerName)
	}

}
