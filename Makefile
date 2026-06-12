BINARY=bin/go-ralph

.PHONY: build test vet install clean

build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/go-ralph/

test:
	go test ./...

vet:
	go vet ./...

install: build
	sudo cp $(BINARY) /usr/local/bin/go-ralph
	@echo "installed to /usr/local/bin/go-ralph"

clean:
	rm -rf bin

test-integration:
	go test ./test/integration/

test-integration-live:
	go test -tags integration ./test/integration/