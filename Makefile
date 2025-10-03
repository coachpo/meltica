GO111MODULE=on

.PHONY: test lint vet tidy build standards ci clean

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
