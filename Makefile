BINARY := jsondiffp
BUILD_DIR := bin
UNAME_S := $(shell uname -s)
BREW_PREFIX := $(shell command -v brew >/dev/null 2>&1 && brew --prefix 2>/dev/null)
ifeq ($(UNAME_S),Darwin)
PREFIX ?= $(if $(BREW_PREFIX),$(BREW_PREFIX),/usr/local)
else
PREFIX ?= /usr/local
endif
BINDIR ?= $(PREFIX)/bin
GOFLAGS ?=
LEFT ?= a.json
RIGHT ?= b.json
PRECISION ?=
JSON ?=
ROUND ?=

.PHONY: help fmt test check build run install uninstall clean

help:
	@printf 'Targets:\n'
	@printf '  fmt     format Go files\n'
	@printf '  test    run Go tests\n'
	@printf '  check   format and test\n'
	@printf '  build   build CLI to $(BUILD_DIR)/$(BINARY)\n'
	@printf '  run     run CLI: make run LEFT=a.json RIGHT=b.json PRECISION=2 JSON=1 ROUND=1\n'
	@printf '  install install CLI to $(DESTDIR)$(BINDIR)/$(BINARY)\n'
	@printf '  uninstall remove CLI from $(DESTDIR)$(BINDIR)/$(BINARY)\n'
	@printf '          PREFIX defaults to brew --prefix on macOS when Homebrew is available\n'
	@printf '  clean   remove build artifacts\n'

fmt:
	gofmt -w .

test:
	go test $(GOFLAGS) ./...

check: fmt test

build:
	mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY) .

run:
	go run $(GOFLAGS) . $(if $(JSON),-j) $(if $(ROUND),-r) $(if $(PRECISION),-p $(PRECISION)) $(LEFT) $(RIGHT)

install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 $(BUILD_DIR)/$(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)