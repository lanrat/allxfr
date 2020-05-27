# creates static binaries
CC := CGO_ENABLED=0 go build -ldflags "-w -s" -trimpath -a -installsuffix cgo

SOURCES := $(shell find . -maxdepth 1 -type f -name '*.go')
BIN := allxfr

.PHONY: all fmt docker docker-unbound clean

all: allxfr

docker-unbound: unbound/Dockerfile
	docker build -t="lanrat/unbound" unbound/

run-unbound:
	docker run -it --rm --name unbound -p 127.0.0.1:5053:5053/udp lanrat/unbound

docker: Dockerfile
	docker build -t="lanrat/allxfr" .

$(BIN): $(SOURCES) $(MODULE_SOURCES) go.mod go.sum
	$(CC) -o $@ $(SOURCES)

check:
	golangci-lint run
	staticcheck -unused.whole-program -checks all ./...

clean:
	rm $(BIN)

fmt:
	gofmt -s -w -l .
