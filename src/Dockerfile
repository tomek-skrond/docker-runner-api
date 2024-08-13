# Build stage
FROM golang:alpine as builder
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY *.go ./
COPY templates ./templates

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o server

# Final stage
FROM alpine
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/server ./server
COPY --from=builder /app/ ./

# Expose the port
EXPOSE 7777

# Start the application
CMD ["./server"]