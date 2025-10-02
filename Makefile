GO111MODULE=on

.PHONY: test lint tidy build standards ci clean

lint:
	golangci-lint run || true

test:
	go test ./... -race -count=1

build:
	go build -o out/ ./...


ci:
	$(MAKE) test
	$(MAKE) build

standards:
	$(MAKE) test

tidy:
	go mod tidy

clean:
	rm -rf out/
