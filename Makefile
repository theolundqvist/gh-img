.PHONY: build test vet lint fmt install clean

build:
	go build -o gh-img ./cmd/gh-img

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	gofumpt -w .

install:
	go install ./cmd/gh-img

clean:
	rm -f gh-img
