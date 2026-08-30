BINARY   := terminal_calendar
BINDIR   := build
GO       := go

.PHONY: all build run test vet fmt clean install

all: build

build:
	$(GO) build -o $(BINDIR)/$(BINARY) .

run: build
	./$(BINDIR)/$(BINARY)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf $(BINDIR)

install:
	$(GO) install .