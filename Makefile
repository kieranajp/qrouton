BIN      := qrouton
EVAL     := qrouton-eval
BINDIR   ?= $(HOME)/.local/bin
FRONTEND := internal/desktop/frontend
PAGES    := internal/desktop/assets/index.html
SHARE    := internal/share/assets/share.js
VERSION  ?= 0.1.0
# The oldest release the version being cut will still talk to. Raise it only
# for a change an older install cannot be left running against — a session
# manifest or control-socket break — because every install below it is held at
# the update gate until it has swapped itself.
MINIMUM_VERSION ?= 0.0.0
BUILD_NUMBER ?= 1
SIGN_IDENTITY ?= -
MACOSX_DEPLOYMENT_TARGET ?= 12.0
export VERSION MINIMUM_VERSION BUILD_NUMBER SIGN_IDENTITY MACOSX_DEPLOYMENT_TARGET
SOURCES  := $(wildcard $(FRONTEND)/*.html $(FRONTEND)/*.js $(FRONTEND)/*/index.html) \
            $(shell find $(FRONTEND)/src -type f 2>/dev/null)

.PHONY: build eval front front-check test race vet fmt check app archive dist install uninstall clean

# The embedded asset tree is generated, and //go:embed fails to compile against
# a directory with nothing in it — so every Go target below depends on `front`.
front: $(PAGES) $(SHARE)

$(FRONTEND)/node_modules: $(FRONTEND)/package-lock.json
	cd $(FRONTEND) && npm ci
	touch $@

$(PAGES): $(FRONTEND)/node_modules $(SOURCES)
	cd $(FRONTEND) && npm run build

# The shared page carries its fonts, so it is built apart from the workbench's
# own pages, which are served theirs.
$(SHARE): $(FRONTEND)/node_modules $(SOURCES)
	cd $(FRONTEND) && npm run build:share

build: front
	go build -o $(BIN) .

app: front
	./build/macos/package.sh

archive:
	./build/macos/archive.sh

dist: app
	$(MAKE) archive

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
	cd $(FRONTEND) && npm run check && npm run test:unit && npm run test:browser

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
	rm -rf internal/desktop/assets internal/share/assets $(FRONTEND)/node_modules
	rm -rf dist
