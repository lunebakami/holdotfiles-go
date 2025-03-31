.PHONY: build test clean deps

deps:
	go get github.com/charmbracelet/bubbletea
	go get github.com/charmbracelet/lipgloss
	go get github.com/charmbracelet/bubbles
	go get github.com/fsnotify/fsnotify

build: deps
	go build -o bin/filesync ./cmd/filesync

test: 
	go test ./...

clean:
	rm -rf bin

run:
	go run ./cmd/filesync

install:
	go install ./cmd/filesync


