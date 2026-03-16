GO ?= go
GOFMT ?= gofmt
GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')

.PHONY: test lint fmt fmt-check race coverage

test:
	$(GO) test ./...

lint:
	@echo "lint placeholder: configure linter when implementation starts"

fmt:
	$(GOFMT) -w $(GOFILES)

fmt-check:
	@unformatted="$$(gofmt -l $(GOFILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "unformatted Go files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

race:
	$(GO) test -race ./...

coverage:
	$(GO) test -coverprofile=coverage.out ./...
