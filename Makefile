gofiles := $(shell find . -name "*.go" -type f -not -path "./vendor/*")

BUILD_TAG = $(shell git tag --points-at HEAD)

BUILD_CONSTS = \
	-X main.buildTime=`date -u '+%Y-%m-%d_%H:%M:%S'` \
	-X main.buildRevision=`git rev-parse HEAD` \
	-X main.buildTag=$(BUILD_TAG)

BUILD_OPTS = -ldflags="$(BUILD_CONSTS)" -gcflags="-trimpath=$(GOPATH)/src"

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

build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) build-prod

release-linux-amd64: build-linux-amd64
	GOOS=linux GOARCH=amd64 $(MAKE) release-prod

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(MAKE) build-prod

release-darwin-amd64: build-darwin-amd64
	GOOS=darwin GOARCH=amd64 $(MAKE) release-prod

build-prod:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_OPTS) -mod=vendor -o bin/vmctl-$(GOOS)-$(GOARCH)

release-prod:
	cd bin && tar czf vmctl-$(BUILD_TAG)-$(GOOS)-$(GOARCH).tar.gz vmctl-$(GOOS)-$(GOARCH) && \
		shasum -a 256 vmctl-$(BUILD_TAG)-$(GOOS)-$(GOARCH).tar.gz >> vmctl-$(BUILD_TAG)_checksums.txt

release: test \
	fmt \
	clean \
	release-darwin-amd64 \
	release-linux-amd64