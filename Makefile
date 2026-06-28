# Output: build/proxy.zip, build/authorizer.zip, build/magic-keepalive.zip

GOOS      := linux
GOARCH    := arm64
BUILD     := build
PROXY     := $(BUILD)/proxy
AUTH      := $(BUILD)/authorizer
KEEPALIVE := $(BUILD)/magic-keepalive

.PHONY: all clean build proxy authorizer keepalive zips changelog version

all: zips

zips: $(BUILD)/proxy.zip $(BUILD)/authorizer.zip $(BUILD)/magic-keepalive.zip

$(BUILD)/proxy.zip: $(PROXY)/bootstrap
	cd $(PROXY) && zip -q ../proxy.zip bootstrap

$(BUILD)/authorizer.zip: $(AUTH)/bootstrap
	cd $(AUTH) && zip -q ../authorizer.zip bootstrap

$(BUILD)/magic-keepalive.zip: $(KEEPALIVE)/bootstrap
	cd $(KEEPALIVE) && zip -q ../magic-keepalive.zip bootstrap

$(PROXY)/bootstrap:
	mkdir -p $(PROXY)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(PROXY)/bootstrap ./cmd/proxy

$(AUTH)/bootstrap:
	mkdir -p $(AUTH)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(AUTH)/bootstrap ./cmd/authorizer

$(KEEPALIVE)/bootstrap:
	mkdir -p $(KEEPALIVE)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(KEEPALIVE)/bootstrap ./cmd/magic-keepalive

build: $(PROXY)/bootstrap $(AUTH)/bootstrap $(KEEPALIVE)/bootstrap

proxy: $(PROXY)/bootstrap
authorizer: $(AUTH)/bootstrap
keepalive: $(KEEPALIVE)/bootstrap

clean:
	rm -rf $(BUILD)

changelog:
	git-cliff -o CHANGELOG.md

version:
	@git-cliff --bumped-version 2>/dev/null || echo "0.1.0"
