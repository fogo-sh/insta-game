.PHONY: build download clean

build:
	docker compose -f ../../compose.yml --profile images build bzflag

download:
	./download.sh

clean:
	./clean.sh
