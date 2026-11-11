GO_BUILD_DIR ?= ./bin

.PHONY: fmt swagger swagger-check
fmt: ## Run go fmt against code.
	go fmt ./...

swagger: ## Generate Swagger 2.0 documentation from API annotations.
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo api/api.go --output docs --propertyStrategy pascalcase

swagger-check: swagger ## Verify generated Swagger documentation is committed.
	git diff --exit-code -- docs

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
