BINARY_NAME=runner


build:
	GOARCH=amd64 GOOS=linux go build -o $(BINARY_NAME) .

run: build
	. ./.env && ./$(BINARY_NAME)

cleanup:
	go clean
	rm $(BINARY_NAME)

