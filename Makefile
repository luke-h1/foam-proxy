# Output: build/proxy.zip, build/authorizer.zip

GOOS   := linux
GOARCH := arm64
BUILD  := build
PROXY  := $(BUILD)/proxy
AUTH   := $(BUILD)/authorizer

.PHONY: all clean build proxy authorizer zips changelog version

all: zips

zips: $(BUILD)/proxy.zip $(BUILD)/authorizer.zip

$(BUILD)/proxy.zip: $(PROXY)/bootstrap
	cd $(PROXY) && zip -q ../proxy.zip bootstrap

$(BUILD)/authorizer.zip: $(AUTH)/bootstrap
	cd $(AUTH) && zip -q ../authorizer.zip bootstrap

$(PROXY)/bootstrap:
	mkdir -p $(PROXY)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(PROXY)/bootstrap ./cmd/proxy

$(AUTH)/bootstrap:
	mkdir -p $(AUTH)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(AUTH)/bootstrap ./cmd/authorizer

build: $(PROXY)/bootstrap $(AUTH)/bootstrap

proxy: $(PROXY)/bootstrap
authorizer: $(AUTH)/bootstrap

clean:
	rm -rf $(BUILD)

changelog:
	git-cliff -o CHANGELOG.md

version:
	@git-cliff --bumped-version 2>/dev/null || echo "0.1.0"
