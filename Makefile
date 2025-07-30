default: allxfr

include release.mk

# creates static binaries
CC := CGO_ENABLED=0 go build -ldflags "-w -s -X main.version=${VERSION}" -trimpath -a -installsuffix cgo

SOURCES := $(shell find . -type f -name '*.go')
BIN := allxfr

.PHONY: all fmt docker  clean deps update-deps

docker: Dockerfile $(SOURCES) go.mod go.sum
	docker build --build-arg VERSION=${VERSION} -t="lanrat/allxfr" .

$(BIN): $(SOURCES) go.mod go.sum
	$(CC) -o $@ 

check:
	golangci-lint run
	staticcheck -checks all ./...

update-deps: go.mod
	GOPROXY=direct go get -u ./...
	go mod tidy

deps: go.mod
	go mod download

test:
	go test -timeout 15s -v ./...

clean:
	rm $(BIN)

fmt:
	go fmt .

