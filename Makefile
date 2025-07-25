# creates static binaries
CC := CGO_ENABLED=0 go build -ldflags "-w -s" -trimpath -a -installsuffix cgo

SOURCES := $(shell find . -type f -name '*.go')
BIN := allxfr

.PHONY: all fmt docker docker-unbound clean deps update-deps

all: allxfr

docker-unbound: unbound/Dockerfile
	docker build -t="lanrat/unbound" unbound/

run-unbound:
	docker run -it --rm --name unbound -p 127.0.0.1:5053:5053/udp lanrat/unbound

docker: Dockerfile $(SOURCES) go.mod go.sum
	docker build -t="lanrat/allxfr" .

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

