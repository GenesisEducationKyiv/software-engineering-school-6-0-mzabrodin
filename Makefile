.PHONY: build test test-integration lint lint-integration fmt

build:
	go build ./...

test:
	go test -v -count=1 ./...

test-integration:
	go test -v -count=1 -tags integration ./test/integration/...

lint:
	golangci-lint run

lint-integration:
	golangci-lint run --build-tags integration ./test/integration/...

fmt:
	golangci-lint fmt