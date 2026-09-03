BIN      := qrouton
EVAL     := qrouton-eval
BINDIR   ?= $(HOME)/.local/bin
FRONTEND := internal/desktop/frontend
PAGES    := internal/desktop/assets/index.html
BRIDGE   := internal/desktop/frontend/src/lib/bridge/generated.js
SHARE    := internal/share/assets/share.js
VERSION  ?= 0.1.0
BUILD_NUMBER ?= 1
SIGN_IDENTITY ?= -
MACOSX_DEPLOYMENT_TARGET ?= 12.0
export VERSION BUILD_NUMBER SIGN_IDENTITY MACOSX_DEPLOYMENT_TARGET
BOUND    := $(wildcard internal/desktop/*.go internal/status/*.go internal/desktop/bridge/*.go)
SOURCES  := $(wildcard $(FRONTEND)/*.html $(FRONTEND)/*.js $(FRONTEND)/*/index.html) \
            $(shell find $(FRONTEND)/src -type f 2>/dev/null)

.PHONY: build eval front front-check comment-check test race vet fmt check app archive dist install uninstall clean

# The embedded asset tree is generated, and //go:embed fails to compile against
# a directory with nothing in it — so every Go target below depends on `front`.
front: $(PAGES) $(SHARE)

# The page calls the workbench by name, and the names are Go's. Generated before
# the pages that import them, so a rename cannot reach a build.
$(BRIDGE): $(BOUND) go.mod
	go generate ./internal/desktop/bridge/

$(FRONTEND)/node_modules: $(FRONTEND)/package-lock.json
	cd $(FRONTEND) && npm ci
	touch $@

$(PAGES): $(FRONTEND)/node_modules $(BRIDGE) $(SOURCES)
	cd $(FRONTEND) && npm run build

# The shared page carries its fonts, so it is built apart from the workbench's
# own pages, which are served theirs.
$(SHARE): $(FRONTEND)/node_modules $(BRIDGE) $(SOURCES)
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

# A component's props are a contract with the screen that draws it, and a
# bundler will happily ship a page that passes the wrong ones.
front-check: $(FRONTEND)/node_modules
	cd $(FRONTEND) && npm run check && npm run test:unit && npm run test:browser

comment-check: $(FRONTEND)/node_modules
	go run ./cmd/commentdiscipline -policy comment-discipline.json -root .
	cd $(FRONTEND) && npm run test:comments && npm run check:comments

# The pre-handoff gate from AGENTS.md. gofmt -l reports rather than exits, so
# it is asserted empty here.
check: comment-check test race vet build front-check
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
