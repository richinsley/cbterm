.PHONY: all build clean run dev frontend webkit-download

BINARY := bin/cbterm
REPO := calobozan/cbterm

all: build

# Build everything (frontend + Go binary)
build: frontend
	@mkdir -p bin
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/cbterm
	@echo "Built $(BINARY)"

# Run the terminal
run: build
	./$(BINARY)

# Run in debug mode
run-debug: build
	./$(BINARY) --debug

# Development mode: run frontend dev server + Go binary
dev:
	@echo "Starting development mode..."
	@echo "1. Run 'cd frontend && npm run dev' in another terminal"
	@echo "2. Then run: ./$(BINARY) --dev --debug"

# Build frontend (xterm.js)
frontend:
	@if [ ! -d "frontend/node_modules" ]; then \
		echo "Installing frontend dependencies..."; \
		cd frontend && npm install; \
	fi
	@echo "Building frontend..."
	@cd frontend && npm run build

# Clean frontend and rebuild
frontend-clean:
	rm -rf frontend/node_modules frontend/dist
	$(MAKE) frontend

# Download prebuilt WebKit from GitHub releases
webkit-download:
	@echo "Downloading WebKit from latest release..."
	@mkdir -p webkit/lib
	@gh release download --repo $(REPO) --pattern 'webkit-macos-arm64.tar.gz' --dir webkit/lib --clobber 2>/dev/null || \
		(echo "No releases found. Run 'gh workflow run webkit-build.yml' to trigger a build." && exit 1)
	@cd webkit/lib && tar -xzf webkit-macos-arm64.tar.gz && rm webkit-macos-arm64.tar.gz
	@echo "WebKit frameworks extracted to webkit/lib/"

# Trigger WebKit build on GitHub Actions
webkit-build:
	gh workflow run webkit-build.yml --repo $(REPO)
	@echo "WebKit build triggered. Monitor at: https://github.com/$(REPO)/actions"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf frontend/dist/

# Full clean (including node_modules)
distclean: clean
	rm -rf frontend/node_modules
	rm -rf webkit/lib/*.framework

# Show help
help:
	@echo "cbterm - AI-native terminal emulator"
	@echo ""
	@echo "Commands:"
	@echo "  make build          - Build frontend and Go binary"
	@echo "  make run            - Build and run"
	@echo "  make run-debug      - Build and run with dev tools enabled"
	@echo "  make dev            - Instructions for development mode"
	@echo "  make frontend       - Build frontend only"
	@echo "  make webkit-download - Download prebuilt WebKit"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make distclean      - Clean everything including node_modules"
