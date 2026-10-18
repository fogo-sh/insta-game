.PHONY: build test check

build:
	go build -o bin/insta-game ./cmd/insta-game

test:
	go test -race ./...

check:
	go vet ./...
	python3 generate-compose.py --check
