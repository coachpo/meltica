GO111MODULE=on

.PHONY: test lint tidy build build-meltilint meltilint standards ci clean

lint:
	golangci-lint run || true

test:
	go test ./... -race -count=1

build:
	go build -o out/ ./...

build-meltilint:
	go build -o out/meltilint ./internal/meltilint/cmd/meltilint

meltilint:
	go run ./internal/meltilint/cmd/meltilint ./core ./providers/...

ci:
	$(MAKE) test
	$(MAKE) build
	$(MAKE) meltilint

standards:
	$(MAKE) meltilint

tidy:
	go mod tidy

clean:
	rm -rf out/
