# Output: build/proxy.zip, build/authorizer.zip, build/magic-keepalive.zip

GOOS      := linux
GOARCH    := arm64
BUILD     := build
PROXY     := $(BUILD)/proxy
AUTH      := $(BUILD)/authorizer
KEEPALIVE := $(BUILD)/magic-keepalive

.PHONY: all clean build proxy authorizer keepalive zips changelog version run-local invoke-local

# Local emulation (AWS Lambda RIE via docker). Build for the host arch so the
# bootstrap runs natively in the container - no qemu.
LOCAL      := $(BUILD)/local
LOCAL_PORT ?= 9000
CMD        ?= proxy
EVENT      ?= events/proxy-healthcheck.json
RIE_IMAGE  := public.ecr.aws/lambda/provided:al2023
LOCAL_ARCH := $(shell uname -m)
ifeq ($(LOCAL_ARCH),x86_64)
  LOCAL_GOARCH := amd64
else
  LOCAL_GOARCH := arm64
endif

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

# Build a host-arch bootstrap for local emulation, e.g. `make CMD=authorizer run-local`.
$(LOCAL)/$(CMD)/bootstrap: $(shell find ./cmd/$(CMD) -name '*.go') $(COMMON_SRCS)
	mkdir -p $(LOCAL)/$(CMD)
	GOOS=linux GOARCH=$(LOCAL_GOARCH) go build -o $(LOCAL)/$(CMD)/bootstrap ./cmd/$(CMD)

# Start the Lambda under the RIE. Reads .env.local if present. Leave running,
# then in another shell: `make invoke-local` (or with EVENT=events/<name>.json).
run-local: $(LOCAL)/$(CMD)/bootstrap
	docker run --rm -p $(LOCAL_PORT):8080 \
		$(if $(wildcard .env.local),--env-file .env.local,) \
		-v "$(CURDIR)/$(LOCAL)/$(CMD)":/var/task \
		--entrypoint /usr/local/bin/aws-lambda-rie \
		$(RIE_IMAGE) /var/task/bootstrap

# POST an event fixture to a running run-local. Override with EVENT=<path>.
invoke-local:
	curl -s http://localhost:$(LOCAL_PORT)/2015-03-31/functions/function/invocations \
		-d @$(EVENT) | (jq . 2>/dev/null || cat)

clean:
	rm -rf $(BUILD)

changelog:
	git-cliff -o CHANGELOG.md

version:
	@git-cliff --bumped-version 2>/dev/null || echo "0.1.0"
