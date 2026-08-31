BINARY   := terminal_calendar
BINDIR   := build
GO       := go

.PHONY: all build run test alltest vet fmt clean install

all: build

build:
	$(GO) build -o $(BINDIR)/$(BINARY) .

run: build
	./$(BINDIR)/$(BINARY)

test:
	$(GO) test ./...

alltest: ## run the whole suite fresh, ignoring the test cache (mirrors CI)
	$(GO) test -count=1 ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf $(BINDIR)

install:
	$(GO) install .