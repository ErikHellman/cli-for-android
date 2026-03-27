VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS   = -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -s -w"
BINARY    = acli
BUILD_DIR = dist

.PHONY: build install test lint clean release doctor

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/acli

install:
	go install $(LDFLAGS) ./cmd/acli

test:
	go test ./... -v -count=1

lint:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR)

release:
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64   ./cmd/acli
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64   ./cmd/acli
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64    ./cmd/acli
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64    ./cmd/acli
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe ./cmd/acli

doctor:
	@echo "==> Go toolchain"
	@go version
	@echo "==> Module"
	@cat go.mod | head -3
