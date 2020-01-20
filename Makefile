clean:
	rm -rf bin/*

fmt:
	GO111MODULE=on gofmt -l -w -s .

vet:
	GO111MODULE=on go vet ./...

lint: install-golint
	golint
	golint vm/...
	golint influx/...

install-golint:
	which golint || GO111MODULE=off go get -u golang.org/x/lint/golint

test:
	GO111MODULE=on go test -race -v ./...

build:
	go build -mod=vendor -o bin/vmctl
