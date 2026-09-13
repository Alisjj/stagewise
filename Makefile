BINARY ?= stagewise

.PHONY: build test vet clean

build:
	go build -o $(BINARY) ./cmd/stagewise

install:
	go install ./cmd/stagewise

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
