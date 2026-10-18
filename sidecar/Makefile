BINARY=sidecar
GOOS=linux
GOARCH=arm64
CGO_ENABLED=0

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) go build -o $(BINARY) .

build-local:
	go build -o $(BINARY)-local .

test:
	cd .. && go test ./internal/agent ./sidecar/...

vet:
	cd .. && go vet ./internal/agent ./sidecar/...

clean:
	rm -f $(BINARY) $(BINARY)-local
