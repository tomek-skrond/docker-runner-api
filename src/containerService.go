package main

import (
	"context"
	"encoding/json"
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

type ContainerService struct {
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
	startopts types.ContainerStartOptions) *ContainerService {
	return &ContainerService{
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

type ContainerData struct {
	Status          string                    `json:"container_status"`
	ContainerConfig *container.Config         `json:"container_config"`
	HostConfig      *container.HostConfig     `json:"host_config"`
	NetworkConfig   *network.NetworkingConfig `json:"network_config"`
}

func NewContainerData(status string, config *container.Config, hostconf *container.HostConfig, networkconf *network.NetworkingConfig) *ContainerData {
	return &ContainerData{
		Status:          status,
		ContainerConfig: config,
		HostConfig:      hostconf,
		NetworkConfig:   networkconf,
	}
}

func (r *ContainerService) InitializeClient() error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Pull error")
		return err
	}
	r.Client = cli
	defer r.Client.Close()

	return nil
}

func (r *ContainerService) PullDockerImage() (io.ReadCloser, error) {

	out, err := r.Client.ImagePull(r.Context, r.Image, r.PullOpts)
	if err != nil {
		log.Printf("Error pulling image %s\n", err)
		return nil, err
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

func (r *ContainerService) CreateNetwork() (types.NetworkCreateResponse, error) {
	return r.Client.NetworkCreate(
		r.Context,
		r.NetworkName,
		types.NetworkCreate{},
	)
}
func (r *ContainerService) CreateContainer() (container.CreateResponse, error) {
	return r.Client.ContainerCreate(
		r.Context,
		&r.Config,
		&r.HostConf,
		&r.NetConf,
		&r.Platform,
		r.ContainerName)
}

func (r *ContainerService) StartContainer(resp container.CreateResponse) error {
	return r.Client.ContainerStart(r.Context, resp.ID, r.StartOpts)
}

func (r *ContainerService) Containerize() (*ContainerData, error) {

	if err := r.InitializeClient(); err != nil {
		log.Printf("init client error")
		log.Printf("Error initializing client %s\n", err)
		return NewContainerData("client_init_error", &r.Config, &r.HostConf, &r.NetConf), err
	}
	log.Printf("initialized client")

	out, err := r.PullDockerImage()
	if err != nil {
		log.Println(out)
		log.Printf("Error pulling image %s\n", err)
		return NewContainerData("image_pull_error", &r.Config, &r.HostConf, &r.NetConf), err
	}

	netresp, err := r.CreateNetwork()
	if err != nil {
		log.Printf("Error creating network %s\n", err)
		return NewContainerData("network_create_error", &r.Config, &r.HostConf, &r.NetConf), err
	}
	log.Printf("Created network with name %v and ID: %v\n", r.NetworkName, netresp.ID)

	resp, err := r.CreateContainer()
	if err != nil {
		log.Printf("Error creating container %s\n", err)
		return NewContainerData("container_create_error", &r.Config, &r.HostConf, &r.NetConf), err
	}

	if err := r.StartContainer(resp); err != nil {
		log.Printf("Error starting container %s\n", err)
		return NewContainerData("container_start_error", &r.Config, &r.HostConf, &r.NetConf), err
	}
	log.Printf("ID of created container: %s\n", resp.ID)

	return NewContainerData("container_started", &r.Config, &r.HostConf, &r.NetConf), nil
}

func (r *ContainerService) StopContainer() (*ContainerData, error) {
	if err := r.InitializeClient(); err != nil {
		log.Printf("Error initializing client %s\n", err)
		return NewContainerData("client_init_error", &r.Config, &r.HostConf, &r.NetConf), err
	}

	noWaitTimeout := 0
	containerFilters := filters.NewArgs()
	containerFilters.Add("name", r.ContainerName)

	networkFilters := filters.NewArgs()
	networkFilters.Add("name", r.NetworkName)

	containers, err := r.Client.ContainerList(r.Context, types.ContainerListOptions{Filters: containerFilters})
	if err != nil {
		log.Printf("Error listing containers %s\n", err)
		return NewContainerData("container_list_error", &r.Config, &r.HostConf, &r.NetConf), err
	}
	networks, err := r.Client.NetworkList(r.Context, types.NetworkListOptions{Filters: networkFilters})

	if err != nil {
		log.Printf("Error listing networks %s\n", err)
		return NewContainerData("network_list_error", &r.Config, &r.HostConf, &r.NetConf), err
	}

	if len(containers) == 0 && len(networks) == 0 {
		log.Printf("Container does not exist\n")
		return NewContainerData("container_does_not_exist_error", &r.Config, &r.HostConf, &r.NetConf), nil
	}

	if len(containers) == 1 {
		log.Printf("container ID found: %s", containers[0].ID)
		if err := r.Client.ContainerStop(r.Context, r.ContainerName, container.StopOptions{Timeout: &noWaitTimeout}); err != nil {
			log.Printf("Error stopping container %s\n", err)
			return NewContainerData("container_stop_error", &r.Config, &r.HostConf, &r.NetConf), err
		}
		log.Printf("Success stopping container %s\n", r.ContainerName)

	}

	if len(networks) == 1 {
		log.Printf("network ID found: %s", networks[0].ID)
		if err := r.Client.NetworkRemove(r.Context, r.NetworkName); err != nil {
			log.Printf("Error removing network %s\n", err)
			return NewContainerData("network_remove_error", &r.Config, &r.HostConf, &r.NetConf), err
		}
		log.Printf("Success removing network %s\n", r.NetworkName)
	}
	return NewContainerData("container_stopped", &r.Config, &r.HostConf, &r.NetConf), err

}
