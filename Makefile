APP=sshkeeper

.PHONY: build run test vet fmt clean install

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
