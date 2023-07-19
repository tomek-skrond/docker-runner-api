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

Endpoint `/` is the homepage.

Endpoint `/start` runs the container with a server.

Endpoint `/stop` stops the container.

Endpoint `/logs` is an endpoint for reading logs.
