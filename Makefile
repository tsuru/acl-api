GO_BUILD_DIR ?= ./bin

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

.PHONY: build
build: build-dirs
	CGO_ENABLED=0 go build -o $(GO_BUILD_DIR)/

.PHONY: build-dirs
build-dirs:
	@mkdir -p $(GO_BUILD_DIR)
