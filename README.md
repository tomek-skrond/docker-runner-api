# docker-runner-api

This is an application that runs docker container with a Minecraft Server and lets you manage the container's state (Starting/stopping) by using a web interface.


### Running the app

Building the app:
```
go build -o runner && ./runner
```

The best way to run the app is creating an `.env` file and running a task in `Makefile`, that sources environmental vars, builds and runs the code.

```
make run
```


### Endpoints

```
	r.HandleFunc("/stop", s.Stop).Methods("POST")
	r.HandleFunc("/start", s.Start).Methods("POST")
	r.HandleFunc("/", s.Home).Methods("GET")
	r.HandleFunc("/logs", s.Logs)
```

- Endpoint `/` is the homepage.

- Endpoint `/start` runs the container with a server.

- Endpoint `/stop` stops the container.

- Endpoint `/logs` is an endpoint for reading logs.

### Dependencies

To use this application, you have to install [Go](https://go.dev/doc/install) and [Docker](https://docs.docker.com/engine/install/) on your machine:

```
$ go version
go version go1.23.0 linux/amd64
```

```
$ docker version
Client: Docker Engine - Community
 Version:           26.1.0
 API version:       1.45
 Go version:        go1.21.9
 Git commit:        9714adc
 Built:             Mon Apr 22 17:08:20 2024
 OS/Arch:           linux/amd64
 Context:           default

Server: Docker Engine - Community
 Engine:
  Version:          26.1.0
  API version:      1.45 (minimum version 1.24)
  Go version:       go1.21.9
  Git commit:       c8af8eb
  Built:            Mon Apr 22 17:06:36 2024
  OS/Arch:          linux/amd64
  Experimental:     false
 containerd:
  Version:          1.6.31
  GitCommit:        e377cd56a71523140ca6ae87e30244719194a521
 runc:
  Version:          1.1.12
  GitCommit:        v1.1.12-0-g51d5e94
 docker-init:
  Version:          0.19.0
  GitCommit:        de40ad0
```

If you want to use the "Sync with Cloud" functionality, you have to first configure Google Cloud:
- Service account (with roles for bucket operations and service account token creation)
- Application default credentials

ADC could be configured using this command:
```
gcloud auth application-default login --impersonate-service-account $GCP_SERVICE_ACCOUNT
```