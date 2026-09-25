.PHONY: build test vet fmt clean deps run install

deps:
	go mod download

build:
	go build -o bin/hdt ./cmd/hdt

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

clean:
	rm -rf bin

run:
	go run ./cmd/hdt

install:
	sh ./install.sh
