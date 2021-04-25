FROM golang:latest
# A Debian base image with the latest Go and GOPATH set to /go

# Dev tools (can be removed for production)
RUN apt update && apt install -y vim curl jq

# Copy in the source files
COPY ./src/* /go/src/
WORKDIR /go/src

# Get dependencies
RUN go get "github.com/j-keck/arping"
RUN go mod download "github.com/j-keck/arping"

# Build the binary
RUN go build -o /go/bin/arpmon main.go

# Run the service
CMD /go/bin/arpmon


