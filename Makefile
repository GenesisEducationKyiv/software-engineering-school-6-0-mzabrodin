.PHONY: build test lint fmt

build:
	go build ./...

test:
	go test -v ./...

lint:
	golangci-lint run

fmt:
	golangci-lint fmt