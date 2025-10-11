GO111MODULE=on

.PHONY: test bench lint vet tidy build build-linux-arm64 standards ci clean binance-ws-test lint-layers coverage-architecture coverage-full coverage-gates contract-ws-routing

lint:
	golangci-lint run || true

vet:
	go vet ./...

test:
	go test ./... -race -count=1

bench:
	go test -bench . -benchmem ./...

contract-ws-routing:
	go test ./tests/contract/ws-routing -race -count=1

build:
	go build -o out/ ./...

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o out/linux-arm64/ ./...

ci:
	$(MAKE) vet
	$(MAKE) test
	$(MAKE) build

standards:
	$(MAKE) vet
	$(MAKE) test

tidy:
	go mod tidy

lint-layers:
	go test ./tests/architecture -run TestLayerBoundaries -count=1

coverage-architecture:
	mkdir -p coverage
	go test ./tests/architecture -covermode=atomic -count=1 -coverprofile=coverage/architecture.out

coverage-full:
	mkdir -p coverage
	go test ./... -covermode=atomic -coverprofile=coverage/full.out

coverage-gates: coverage-full
	go run ./internal/tools/coveragecheck -profile coverage/full.out -allowlist specs/008-architecture-requirements-req/coverage-allowlist.txt

clean:
	rm -rf out/
	rm -rf bin/
