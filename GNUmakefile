MISE       := $(HOME)/.local/bin/mise
GOEXEC     := $(MISE) exec --
BINARY     := terraform-provider-northflank
GOBIN      ?= $(shell $(HOME)/.local/bin/mise exec -- go env GOPATH)/bin

default: build

.PHONY: build
build:
	$(GOEXEC) go build -o $(BINARY) .

.PHONY: install
install:
	$(GOEXEC) go build -o $(GOBIN)/$(BINARY) .

.PHONY: generate
generate:
	$(GOEXEC) go generate ./...

.PHONY: fmt
fmt:
	$(GOEXEC) gofmt -s -w .

.PHONY: vet
vet:
	$(GOEXEC) go vet ./...

.PHONY: test
test:
	$(GOEXEC) go test ./... -v -count=1 -timeout 30s

.PHONY: testacc
testacc:
	TF_ACC=1 $(GOEXEC) go test ./... -v -count=1 -timeout 120m

.PHONY: docs
docs:
	$(GOEXEC) tfplugindocs generate --provider-name northflank

.PHONY: tidy
tidy:
	$(GOEXEC) go mod tidy

.PHONY: clean
clean:
	rm -f $(BINARY)
