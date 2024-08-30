# mcmgmt-api

This is an application that runs docker container with a Minecraft Server and lets you manage the container's state (Starting/stopping/backup) by using a web API.



## API Specification
App runs with swagger endpoint `/swagger/index.html`. You can see the specs here.

All API JSON responses adhere to the following template:
```
{
  "http_status": <int>,
  "message": "string",
  "response": <data>
}
```

The "response" part handles response specific to the API endpoint like custom JSON structures defined in the code.



## Running the app
Environment vars needed to run the app:
```
#!/bin/bash

 
export ADMIN_USER=username
export ADMIN_PASSWORD=password
export BACKUPS_BUCKET=bucket_name
export PROJECT_ID=gcp_project_id
export JWT_SECRET=very_complicated_password

# if you want to use cloud sync
export GCP_SERVICE_ACCOUNT=svcaccount_with_gcs_access@something.iam.gserviceaccount.com
```

Building the app:
```
go build -o mcmgmt
```

## Dependencies


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

### Google Cloud Platform

If you want to use the "Sync with Cloud" functionality, you have to first configure Google Cloud:
- Service account (with roles for bucket operations and service account token creation)
- Application default credentials

ADC could be configured using this command:
```
gcloud auth application-default login --impersonate-service-account $GCP_SERVICE_ACCOUNT
```

