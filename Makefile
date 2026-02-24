APP_NAME := macback
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/hiiamtrong/macback/internal/cli.Version=$(VERSION)"

.PHONY: build test lint clean install

build:
	go build $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/macback/

test:
	go test -v -race -cover ./...

test-coverage:
	go test -v -race -coverprofile=cover.out ./...
	go tool cover -html=cover.out -o cover.html

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ cover.out cover.html

install: build
	cp bin/$(APP_NAME) /usr/local/bin/

release:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(APP_NAME)-darwin-arm64 ./cmd/macback/
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(APP_NAME)-darwin-amd64 ./cmd/macback/
