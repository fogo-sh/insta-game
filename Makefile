.PHONY: build test check

build:
	go build -o bin/insta-game ./cmd/insta-game

test:
	go test -race ./...
	node --test internal/control/settings_test.mjs

check:
	go vet ./...
	python3 generate-compose.py --check

.PHONY: images
images:
	docker compose --profile images build
