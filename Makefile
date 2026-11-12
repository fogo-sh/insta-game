.PHONY: build test test-browser check lint-templates dist ui

build:
	go build -o bin/insta-game ./cmd/insta-game

test:
	npm --prefix web run check
	go test -race ./...
	npm --prefix web test

ui:
	npm --prefix web run build

test-browser:
	uv run --no-project --with playwright==1.65.1 python scripts/test-lobby-browser.py

check: lint-templates
	go vet ./...
	python3 generate-compose.py --check

.PHONY: images
images:
	docker compose --profile images build

lint-templates:
	uv run --no-project --with cfn-lint==1.60.0 env INSTA_GAME_LINT_TEMPLATES=1 go test ./internal/cloud -run '^TestCloudFormationTemplates$$' -count=1

dist:
	python3 scripts/package-release.py
