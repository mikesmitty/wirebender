TINYGO ?= tinygo
TARGET ?= pico2
SERIAL ?= usb
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date "+%Y-%m-%d_%H:%M:%S")

LDFLAGS := -X main.version=$(VERSION) -X main.gitHash=$(GIT) -X main.buildTime=$(DATE)

.PHONY: flash build test-servo

flash:
	$(TINYGO) flash -target=$(TARGET) -serial=$(SERIAL) -ldflags="$(LDFLAGS)"

OUTPUT ?= build.elf

build:
	$(TINYGO) build -target=$(TARGET) -serial=$(SERIAL) -ldflags="$(LDFLAGS)" -o $(OUTPUT) .

test-servo:
	cd cmd/servotest && go run . -verbose

build-dxf2bend:
	go build -ldflags="-X main.version=$(VERSION)" -o dxf2bend ./cmd/dxf2bend/
