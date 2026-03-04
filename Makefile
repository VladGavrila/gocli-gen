BINARY  := gocli-gen
LDFLAGS := -ldflags "-s -w"

.PHONY: build macos-arm linux-amd64 clean tidy test

build: macos-arm linux-amd64

macos-arm:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-macos-arm64 .

linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .

dev:
	go build -o $(BINARY) .

tidy:
	go mod tidy

test:
	go test ./...

clean:
	rm -rf dist/ $(BINARY)
