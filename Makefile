.PHONY: build test check lint-templates dist

build:
	go build -o bin/insta-game ./cmd/insta-game

test:
	go test -race ./...
	node --test internal/control/settings_test.mjs

check: lint-templates
	go vet ./...
	python3 generate-compose.py --check

.PHONY: images
images:
	docker compose --profile images build

lint-templates:
	uv run --no-project --with cfn-lint==1.58.0 env INSTA_GAME_LINT_TEMPLATES=1 go test ./internal/cloud -run '^TestCloudFormationTemplates$$' -count=1

dist:
	python3 scripts/package-release.py
