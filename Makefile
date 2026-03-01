.PHONY: build build-linux build-all clean test vet

BINARY=numentext
VERSION=1.0.0

build:
	go build -o $(BINARY) .

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 .

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)-darwin-arm64 .

build-all: build build-linux build-darwin-arm64

test:
	go test ./... -count=1

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64 $(BINARY)-darwin-arm64
