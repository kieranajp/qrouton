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

# The Alt-e and Alt-n chords in the vendored Zellij config run `qrouton` from
# PATH, so keyboard escalation only works against an installed binary.
install: build
	mkdir -p $(BINDIR)
	install -m 755 $(BIN) $(BINDIR)/$(BIN)

uninstall:
	rm -f $(BINDIR)/$(BIN)

clean:
	rm -f $(BIN) $(EVAL)
