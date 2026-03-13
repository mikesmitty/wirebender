TINYGO ?= tinygo
GOTOOLCHAIN ?= go1.25.8
TARGET ?= metro-rp2350
SERIAL ?= usb
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date "+%Y-%m-%d_%H:%M:%S")

LDFLAGS := -X main.version=$(VERSION) -X main.gitHash=$(GIT) -X main.buildTime=$(DATE)

.PHONY: flash build test-servo

flash:
	GOTOOLCHAIN=$(GOTOOLCHAIN) $(TINYGO) flash -target=$(TARGET) -serial=$(SERIAL) -ldflags="$(LDFLAGS)"

OUTPUT ?= build.elf

build:
	GOTOOLCHAIN=$(GOTOOLCHAIN) $(TINYGO) build -target=$(TARGET) -serial=$(SERIAL) -ldflags="$(LDFLAGS)" -o $(OUTPUT) .

test-servo:
	cd cmd/servotest && GOTOOLCHAIN=$(GOTOOLCHAIN) go run . -verbose

build-dxf2bend:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go build -ldflags="-X main.version=$(VERSION)" -o dxf2bend ./cmd/dxf2bend/
