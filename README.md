# docker-runner-api

This is an application that runs docker container with a Minecraft Server and lets you manage the container's state (Starting/stopping) by using a web interface.


### Running the app

To run this app successfully, you have to export two environmental variables:

```
export TEMPLATE_PATH=<directory with main.go>/templates/
export LOGS_PATH=<directory with main.go>/mcdata/logs/latest.log
```

`TEMPLATE_PATH` is a path for a web app to find templates.

`LOGS_PATH` is a path to minecraft server's log file.

After exporting envs, you can build and run the application:

```
go build -o runner && ./runner
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
go version go1.20.5 linux/amd64
```

```
$ docker version
Client:
 Version:           24.0.2
 API version:       1.43
 Go version:        go1.20.4
 Git commit:        cb74dfcd85
 Built:             Mon May 29 15:50:06 2023
 OS/Arch:           linux/amd64
 Context:           default

Server:
 Engine:
  Version:          24.0.2
  API version:      1.43 (minimum version 1.12)
  Go version:       go1.20.4
  Git commit:       659604f9ee
  Built:            Mon May 29 15:50:06 2023
  OS/Arch:          linux/amd64
  Experimental:     false
 containerd:
  Version:          v1.7.2
  GitCommit:        0cae528dd6cb557f7201036e9f43420650207b58.m
 runc:
  Version:          1.1.7
  GitCommit:        
 docker-init:
  Version:          0.19.0
  GitCommit:        de40ad0
```