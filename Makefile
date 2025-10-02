GO111MODULE=on

.PHONY: test lint tidy build build-meltilint build-validate-schemas build-barista meltilint validate-schemas conformance conformance-offline standards ci clean

lint:
	golangci-lint run || true

test:
	go test ./... -race -count=1

build:
	go build -o out/ ./...

build-meltilint:
	go build -o out/meltilint ./internal/meltilint/cmd/meltilint

build-validate-schemas:
	go build -o out/validate-schemas ./cmd/validate-schemas

build-barista:
	go build -o out/barista ./cmd/barista

meltilint:
	go run ./internal/meltilint/cmd/meltilint ./core ./providers/...

validate-schemas:
	go run ./cmd/validate-schemas

conformance:
	go test ./conformance/... -count=1

conformance-offline:
	go test ./conformance -run TestOffline -count=1

ci:
	$(MAKE) test
	$(MAKE) build
	$(MAKE) meltilint
	$(MAKE) validate-schemas
	$(MAKE) conformance

standards:
	$(MAKE) meltilint
	$(MAKE) validate-schemas
	$(MAKE) conformance-offline

tidy:
	go mod tidy

clean:
	rm -rf out/
