BINARY := uports 
VERSION := dev

.PHONY: all fmt vet lint build clean

all: fmt vet lint build

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run

build:
	mkdir -p bin
	go build -ldflags="-X main.version=$(VERSION)" -o bin/$(BINARY)

clean:
	rm -rf bin/
