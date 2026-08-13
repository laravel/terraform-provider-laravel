HOSTNAME=registry.terraform.io
NAMESPACE=laravel
NAME=laravel
BINARY=terraform-provider-${NAME}
VERSION?=0.1.0
OS_ARCH=$(shell go env GOOS)_$(shell go env GOARCH)

default: build

build:
	go build -o ${BINARY}

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}/

test:
	go test ./... -count=1 -parallel=4 -timeout 30s

testacc:
	TF_ACC=1 go test ./... -v -count=1 -parallel=4 -timeout 240m

# Plan-check tests run real terraform plan/apply against an in-memory fake API.
# They need TF_ACC=1 and a terraform binary, but no token and no network.
testplan:
	TF_ACC=1 go test ./internal/provider/ -v -count=1 -run 'Plan$$' -timeout 5m

vet:
	@echo "==> Running go vet..."
	@go vet ./... ; if [ $$? -ne 0 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

fmt:
	gofmt -s -w .

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

lint:
	golangci-lint run ./...

generate:
	go generate ./...

vendor:
	go mod tidy
	go mod vendor

test-compile:
	@if [ "$(TEST)" = "./..." ]; then \
		echo "ERROR: Set TEST to a specific package. For example,"; \
		echo "  make test-compile TEST=./internal/provider"; \
		exit 1; \
	fi
	go test -c $(TEST) -o /dev/null

.PHONY: build install test testacc testplan vet fmt fmtcheck lint generate vendor test-compile
