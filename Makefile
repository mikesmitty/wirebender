TARGET ?= pico2
GIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date "+%Y-%m-%d_%H:%M:%S")

LDFLAGS := -X main.gitHash=$(GIT) -X main.buildTime=$(DATE)

.PHONY: flash build

flash:
	tinygo flash -target=$(TARGET) -ldflags="$(LDFLAGS)"

build:
	tinygo build -o build.elf -target=$(TARGET) -ldflags="$(LDFLAGS)"
