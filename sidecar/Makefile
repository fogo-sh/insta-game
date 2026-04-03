BINARY=sidecar
GOOS=linux
GOARCH=arm64
CGO_ENABLED=0

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) go build -o $(BINARY) .

build-local:
	go build -o $(BINARY)-local .

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(BINARY)-local
