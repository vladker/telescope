.PHONY: all build test clean release

BINARY_NAME := telescope
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

all: build

build:
	go build $(LDFLAGS) -o telescope-encode.exe ./cmd/encode
	go build $(LDFLAGS) -o telescope-decode.exe ./cmd/decode
	go build $(LDFLAGS) -o telescope-recorder.exe ./cmd/recorder

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f telescope-*.exe

release: build
	@mkdir -p release/$(VERSION)
	cp telescope-encode.exe release/$(VERSION)/
	cp telescope-decode.exe release/$(VERSION)/
	cp telescope-recorder.exe release/$(VERSION)/
	@echo "Release $(VERSION) created in release/$(VERSION)/"
