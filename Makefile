BIN      := qrouton
EVAL     := qrouton-eval
BINDIR   ?= $(HOME)/.local/bin
FRONTEND := internal/desktop/frontend
PAGES    := internal/desktop/assets/index.html
SOURCES  := $(wildcard $(FRONTEND)/*.html $(FRONTEND)/*.js $(FRONTEND)/*/index.html) \
            $(shell find $(FRONTEND)/src -type f 2>/dev/null)

.PHONY: build eval front front-check test race vet fmt check install uninstall clean

# The embedded asset tree is generated, and //go:embed fails to compile against
# a directory with nothing in it — so every Go target below depends on `front`.
front: $(PAGES)

$(FRONTEND)/node_modules: $(FRONTEND)/package-lock.json
	cd $(FRONTEND) && npm ci
	touch $@

$(PAGES): $(FRONTEND)/node_modules $(SOURCES)
	cd $(FRONTEND) && npm run build

build: front
	go build -o $(BIN) .

eval:
	go build -o $(EVAL) ./cmd/$(EVAL)

test: front
	go test ./...

race: front
	go test -race ./...

vet: front
	go vet ./...

fmt:
	gofmt -w .

# The pre-handoff gate from AGENTS.md. gofmt -l reports rather than exits, so
# it is asserted empty here.
# A component's props are a contract with the screen that draws it, and a
# bundler will happily ship a page that passes the wrong ones.
front-check: $(FRONTEND)/node_modules
	cd $(FRONTEND) && npm run check && npm test

check: test race vet build front-check
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
	rm -rf internal/desktop/assets $(FRONTEND)/node_modules
