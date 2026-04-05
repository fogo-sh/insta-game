all: download clean build

UV_PROJECT=../../pulumi
RUFF_TARGETS=../../pulumi

download:
	./download.sh

clean:
	./clean.sh

build:
	./build.sh

run:
	./run.sh

ruff:
	uv run --project $(UV_PROJECT) ruff check $(RUFF_TARGETS) --fix --unsafe-fixes
	uv run --project $(UV_PROJECT) ruff format $(RUFF_TARGETS)
