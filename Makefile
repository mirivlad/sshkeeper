APP=sshkeeper
RELEASE_CHECK_DIR ?= /tmp/sshkeeper-release-check

.PHONY: build run test vet fmt clean install release-check

build:
	go build -o bin/$(APP) .

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
	go build -o $(HOME)/.local/bin/$(APP) .

release-check:
	rm -rf $(RELEASE_CHECK_DIR)
	mkdir -p $(RELEASE_CHECK_DIR)
	go test ./...
	go vet ./...
	CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP) .
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-linux-amd64 .
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o $(RELEASE_CHECK_DIR)/$(APP)-windows-amd64.exe .
