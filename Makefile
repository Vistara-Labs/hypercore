BIN_DIR := bin

.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(shell git describe --abbrev=0 --tags)" -o $(BIN_DIR)/containerd-shim-hypercore-example ./cmd
	ln -sf containerd-shim-hypercore-example $(BIN_DIR)/hypercore

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix
