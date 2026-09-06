APP=sshkeeper
VERSION ?= $(shell git describe --tags --match 'v*' --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X github.com/mirivlad/sshkeeper/cmd.Version=$(VERSION)
RELEASE_CHECK_DIR ?= /tmp/sshkeeper-release-check

.PHONY: build run test vet fmt clean install packaging-test release-check

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) .

run:
	go run .

vet:
	go vet ./...

fmt:
	go fmt ./...

test:
	go test ./...

clean:
	rm -rf bin

install:
	go build -ldflags "$(LDFLAGS)" -o $(HOME)/.local/bin/$(APP) .

packaging-test:
	./packaging/scripts/test-legacy-migration.sh

release-check:
	rm -rf $(RELEASE_CHECK_DIR)
	mkdir -p $(RELEASE_CHECK_DIR)
	go test ./...
	go vet ./...
	./packaging/scripts/test-legacy-migration.sh
	CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP) .
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-linux-amd64 .
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-windows-amd64.exe .
