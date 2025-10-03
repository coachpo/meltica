GO111MODULE=on

.PHONY: test lint vet tidy build standards ci clean binance-ws-test

lint:
	golangci-lint run || true

vet:
	go vet ./...

test:
	go test ./... -race -count=1

build:
	go build -o out/ ./...


ci:
	$(MAKE) vet
	$(MAKE) test
	$(MAKE) build

standards:
	$(MAKE) vet
	$(MAKE) test

tidy:
	go mod tidy

clean:
	rm -rf out/

binance-ws-test:
	go build -o bin/ ./cmd/binance-ws-test
	./bin/binance-ws-test

binance-ws-validation:
	go build -o bin/ ./cmd/binance-ws-validation
	./bin/binance-ws-validation

binance-orderbook-validation:
	go build -o bin/ ./cmd/binance-orderbook-validation
	./bin/binance-orderbook-validation

binance-snapshot-test:
	go build -o bin/ ./cmd/binance-snapshot-test
	./bin/binance-snapshot-test
