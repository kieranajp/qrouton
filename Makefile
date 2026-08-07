BIN    := qrouton
EVAL   := qrouton-eval
BINDIR ?= $(HOME)/.local/bin

.PHONY: build eval test race vet fmt check install uninstall clean

build:
	go build -o $(BIN) .

eval:
	go build -o $(EVAL) ./cmd/$(EVAL)

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# The pre-handoff gate from AGENTS.md. gofmt -l reports rather than exits, so
# it is asserted empty here.
check: test race vet build
	@test -z "$$(gofmt -l .)" || { echo "gofmt:"; gofmt -l .; exit 1; }
	git diff --check

# The session shells out to the installed `qrouton` for its own subcommands, so
# workspace actions need an up-to-date binary.
install: build
	mkdir -p $(BINDIR)
	install -m 755 $(BIN) $(BINDIR)/$(BIN)

uninstall:
	rm -f $(BINDIR)/$(BIN)

clean:
	rm -f $(BIN) $(EVAL)
