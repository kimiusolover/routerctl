BINARY := routerctl
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: all build install uninstall test vet fmt check clean layer-check layer-build

all: check build

build:
	go build -trimpath -o bin/$(BINARY) ./cmd/routerctl

# Install only into the current user's prefix by default. Override PREFIX for
# packaging; never require sudo for normal local use.
install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 bin/$(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -type f)

check: test vet

layer-check:
	@test -n "$(PROFILE)" || (echo "usage: make layer-check PROFILE=<profile>" && exit 1)
	./packaging/check-layer.sh $(PROFILE)

layer-build:
	@test -n "$(PROFILE)" || (echo "usage: make layer-build PROFILE=<profile>" && exit 1)
	./packaging/build.sh $(PROFILE)

clean:
	rm -rf bin dist
