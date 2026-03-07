TINYGO ?= tinygo
TARGET ?= pico2
SERIAL ?= usb
GIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date "+%Y-%m-%d_%H:%M:%S")

LDFLAGS := -X main.gitHash=$(GIT) -X main.buildTime=$(DATE)

.PHONY: flash build test-servo

flash:
	$(TINYGO) flash -target=$(TARGET) -serial=$(SERIAL) -ldflags="$(LDFLAGS)"

build:
	$(TINYGO) build -target=$(TARGET) -serial=$(SERIAL) -ldflags="$(LDFLAGS)" -o build.elf .

test-servo:
	cd cmd/servotest && go run . -verbose
