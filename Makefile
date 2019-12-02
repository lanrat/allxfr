# creates static binaries
CC := CGO_ENABLED=0 go build -trimpath -a -installsuffix cgo

#MODULE_SOURCES := $(shell find */ -type f -name '*.go' )
SOURCES := $(shell find . -maxdepth 1 -type f -name '*.go')
BIN := allxfr

.PHONY: all fmt docker clean

all: allxfr

docker: Dockerfile
	docker build -t="lanrat/allxfr" .

$(BIN): $(SOURCES) $(MODULE_SOURCES) go.mod go.sum
	$(CC) -o $@ $(SOURCES)

clean:
	rm $(BIN)

fmt:
	gofmt -s -w -l .
