package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

type ContainerRunner struct {
	Image        string
	BuildContext string
	Context      context.Context
	Client       *client.Client
}

func NewContainerRunner(img string, ctx context.Context, cli *client.Client) *ContainerRunner {
	return &ContainerRunner{
		Image:   img,
		Context: ctx,
		Client:  cli,
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

func (r *ContainerRunner) PullDockerImage(pullopts types.ImagePullOptions) (io.ReadCloser, error) {

	out, err := r.Client.ImagePull(r.Context, r.Image, pullopts)
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

// r.BuildContainer(buildopts)

// Function for building containers
func (r *ContainerRunner) BuildContainer(bopts types.ImageBuildOptions) error {
	imageBuildResponse, err := r.Client.ImageBuild(context.Background(), getBuildContext("./nginx/"), bopts)

	if err != nil {
		log.Fatal(err)
		return err
	}
	defer imageBuildResponse.Body.Close()
	_, err = io.Copy(os.Stdout, imageBuildResponse.Body)
	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func (r *ContainerRunner) CreateContainer(config *container.Config) (container.CreateResponse, error) {
	return r.Client.ContainerCreate(r.Context, config, nil, nil, nil, "")
}

func (r *ContainerRunner) StartContainer(resp container.CreateResponse, startopts *types.ContainerStartOptions) error {
	return r.Client.ContainerStart(r.Context, resp.ID, *startopts)
}

func (r *ContainerRunner) Containerize() {
	pullopts := types.ImagePullOptions{}

	//r := NewContainerRunner("nginx", context.Background(), &client.Client{})

	containerConfig := &container.Config{
		Image: r.Image,
	}

	if err := r.InitializeClient(); err != nil {
		fmt.Println("init client error")
		panic(err)
	}

	out, err := r.PullDockerImage(pullopts)
	if err != nil {
		fmt.Println(out)
		fmt.Println("Pull error")
		panic(err)
	}

	startopts := &types.ContainerStartOptions{}

	resp, err := r.CreateContainer(containerConfig)
	if err := r.StartContainer(resp, startopts); err != nil {
		fmt.Println("start container error")
		panic(err)
	}

	fmt.Println("ID of created container: ", resp.ID)

}

func getBuildContext(path string) io.Reader {
	// Create a tar archive of the Docker build context
	buildContext, err := archive.TarWithOptions(path, &archive.TarOptions{})
	if err != nil {
		log.Fatal(err)
	}
	return buildContext
}
