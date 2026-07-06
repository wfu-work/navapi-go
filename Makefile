APP_NAME ?= navapi
BUILD_DIR ?= build
GO ?= go

GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
CGO_ENABLED ?= $(shell $(GO) env CGO_ENABLED)

EXT := $(if $(filter windows,$(GOOS)),.exe,)
OUTPUT ?= $(BUILD_DIR)/$(APP_NAME)$(EXT)

.PHONY: all build clean test run

all: build

build:
	mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags "-s -w" -o $(OUTPUT) .

test:
	$(GO) test ./...

run:
	$(GO) run .

clean:
	rm -rf $(BUILD_DIR)
