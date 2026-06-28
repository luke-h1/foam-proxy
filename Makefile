# Output: build/proxy.zip, build/authorizer.zip, build/magic-keepalive.zip

GOOS      := linux
GOARCH    := arm64
BUILD     := build
PROXY     := $(BUILD)/proxy
AUTH      := $(BUILD)/authorizer
KEEPALIVE := $(BUILD)/magic-keepalive

.PHONY: all clean build proxy authorizer keepalive zips changelog version

# Rebuild a bootstrap when its sources change, not just when it's missing.
COMMON_SRCS    := $(shell find ./internal -name '*.go') go.mod go.sum
PROXY_SRCS     := $(shell find ./cmd/proxy -name '*.go') $(COMMON_SRCS)
AUTH_SRCS      := $(shell find ./cmd/authorizer -name '*.go') $(COMMON_SRCS)
KEEPALIVE_SRCS := $(shell find ./cmd/magic-keepalive -name '*.go') $(COMMON_SRCS)

all: zips

zips: $(BUILD)/proxy.zip $(BUILD)/authorizer.zip $(BUILD)/magic-keepalive.zip

$(BUILD)/proxy.zip: $(PROXY)/bootstrap
	cd $(PROXY) && zip -q ../proxy.zip bootstrap

$(BUILD)/authorizer.zip: $(AUTH)/bootstrap
	cd $(AUTH) && zip -q ../authorizer.zip bootstrap

$(BUILD)/magic-keepalive.zip: $(KEEPALIVE)/bootstrap
	cd $(KEEPALIVE) && zip -q ../magic-keepalive.zip bootstrap

$(PROXY)/bootstrap: $(PROXY_SRCS)
	mkdir -p $(PROXY)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(PROXY)/bootstrap ./cmd/proxy

$(AUTH)/bootstrap: $(AUTH_SRCS)
	mkdir -p $(AUTH)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(AUTH)/bootstrap ./cmd/authorizer

$(KEEPALIVE)/bootstrap: $(KEEPALIVE_SRCS)
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
