.PHONY: build run clean test install

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X github.com/roman-plevka/j9s/cmd.version=$(VERSION) \
	-X github.com/roman-plevka/j9s/cmd.commit=$(COMMIT) \
	-X github.com/roman-plevka/j9s/cmd.date=$(DATE)"

build:
	go build $(LDFLAGS) -o bin/j9s .

run: build
	./bin/j9s

clean:
	rm -rf bin/

test:
	go test -v ./...

install: build
	cp bin/j9s $(GOPATH)/bin/

tidy:
	go mod tidy

fmt:
	go fmt ./...

lint:
	golangci-lint run
