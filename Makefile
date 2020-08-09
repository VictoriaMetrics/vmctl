gofiles := $(shell find . -name "*.go" -type f -not -path "./vendor/*")

clean:
	rm -rf bin/*

fmt:
	GO111MODULE=on gofmt -l -w -s $(gofiles)

vet:
	GO111MODULE=on go vet ./...

lint: install-golint
	golint
	golint vm/...
	golint influx/...
	golint prometheus/...

install-golint:
	which golint || GO111MODULE=off go get -u golang.org/x/lint/golint

test:
	GO111MODULE=on go test -race -v ./...

build:
	go build -mod=vendor -o bin/vmctl
