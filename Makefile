GO ?= go
GOCACHE ?= $(PWD)/.gocache
GOMODCACHE ?= $(PWD)/.gocache/mod
CACHE_ENV = GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)

.PHONY: all fmt test build plugin clean

all: test

fmt:
	$(GO) fmt ./...

test:
	$(CACHE_ENV) $(GO) test ./...

build:
	$(CACHE_ENV) $(GO) build ./...

plugin:
	$(CACHE_ENV) $(GO) build -o bin/ticketplugin ./cmd/ticketplugin

clean:
	rm -rf $(GOCACHE) bin
