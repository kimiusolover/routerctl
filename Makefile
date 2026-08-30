BINARY := routerctl

.PHONY: all build test vet fmt check clean layer-check layer-build

all: check build

build:
	go build -trimpath -o bin/$(BINARY) ./cmd/routerctl

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
