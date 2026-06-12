.PHONY: build test test-integration lint lint-integration fmt up up-build down proto certs

build:
	go build ./...

certs:
	go run ./scripts/gencerts certs

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

up:
	docker compose up -d

up-build:
	docker compose up -d --build

down:
	docker compose down

proto:
	buf dep update
	buf generate