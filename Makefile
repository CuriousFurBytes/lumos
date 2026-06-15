BINARY := lumos
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/CuriousFurBytes/lumos/internal/cli.Version=$(VERSION)

.PHONY: build test fmt vet lint check install clean

build: ## Build the lumos binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test: ## Run tests with the race detector
	go test -race ./...

fmt: ## Format the code
	gofmt -w .

vet: ## Run go vet
	go vet ./...

lint: ## Run staticcheck
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

check: ## Run the full CI gate locally (fmt check, vet, lint, test)
	@test -z "$$(gofmt -l .)" || (echo "run 'make fmt'" && gofmt -l . && exit 1)
	$(MAKE) vet
	$(MAKE) lint
	$(MAKE) test

install: ## Install lumos into GOBIN
	go install -ldflags "$(LDFLAGS)" .

snapshot: ## Build a local release snapshot with GoReleaser (no publish)
	go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean

next-version: ## Print the next tag (BUMP=prerelease|patch|minor|major|stable)
	@go run ./tools/nextver -latest "$$(git tag --list 'v*' --sort=-v:refname | head -n1)" -bump "$(or $(BUMP),prerelease)"

clean:
	rm -f $(BINARY) coverage.out
	rm -rf dist/
