SHELL := /bin/bash
BIN := bin/asesoria
WEB_DIST := backend/internal/webapp/dist

.PHONY: all build build-backend build-web build-extension embed-web run demo dev-backend dev-web test check clean

all: build

build: build-backend build-extension

# The Go binary embeds the compiled SPA. embed-web copies Vite's output into
# the package that owns //go:embed; the .keep file lets Go compile even before
# the frontend has been built.
build-backend: embed-web
	@mkdir -p bin
	cd backend && go build -o ../$(BIN) ./cmd/asesoria

embed-web: build-web
	rm -rf $(WEB_DIST)
	mkdir -p $(WEB_DIST)
	cp -a web/dist/. $(WEB_DIST)/
	touch $(WEB_DIST)/.keep

build-web:
	cd web && npm ci --no-audit --no-fund && npm run build

build-extension:
	cd extension && npm ci --no-audit --no-fund && npm run build

run: build
	./$(BIN)

demo: build
	./$(BIN) -demo

# Run these in two terminals during development. Vite proxies /api to :8787.
dev-backend:
	cd backend && go run ./cmd/asesoria -dev

dev-web:
	cd web && npm run dev

test:
	cd backend && go test ./...
	cd web && npm run build
	cd extension && npm run check

check:
	cd backend && test -z "$$(gofmt -l .)" && go vet ./...
	cd web && npx vue-tsc --noEmit
	cd extension && npm run check

clean:
	rm -rf bin web/dist extension/dist extension/*.js $(WEB_DIST)
	mkdir -p $(WEB_DIST)
	touch $(WEB_DIST)/.keep
