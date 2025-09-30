GO111MODULE=on

.PHONY: test lint tidy build protolint validate-schemas conformance conformance-offline standards ci

lint:
	golangci-lint run || true

test:
	go test ./... -race -count=1

build:
	go build ./...

protolint:
	go run ./internal/protolint/cmd/protolint ./core ./providers/...

validate-schemas:
	go run ./cmd/validate-schemas

conformance:
	go test ./conformance/... -count=1

conformance-offline:
	go test ./conformance -run TestOffline -count=1

ci:
	$(MAKE) test
	$(MAKE) build
	$(MAKE) protolint
	$(MAKE) validate-schemas
	$(MAKE) conformance

standards:
	$(MAKE) protolint
	$(MAKE) validate-schemas
	$(MAKE) conformance-offline

 tidy:
	go mod tidy
