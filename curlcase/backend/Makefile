VERSION ?= dev
BIN     := bin/curlcase

.PHONY: run test test-race vet build clean

run:
	go run ./cmd/curlcase

test:
	go test ./...

# Needs cgo and a C compiler (e.g. apt install gcc); required before merging runner/store/broker changes.
test-race:
	CGO_ENABLED=1 go test -race ./...

vet:
	go vet ./...

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BIN) ./cmd/curlcase

clean:
	rm -rf bin
