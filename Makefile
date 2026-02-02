.PHONY: all build clean run dev webkit-download frontend

BINARY := bin/term
WEBKIT_VERSION ?= latest
REPO := calobozan/term

all: build

# Build the Go binary
build: frontend
	@mkdir -p bin
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/term

# Run the terminal
run: build
	./$(BINARY)

# Development mode with hot reload (future)
dev:
	@echo "TODO: implement hot reload"

# Build frontend (xterm.js)
frontend:
	@if [ -f frontend/package.json ]; then \
		cd frontend && npm install && npm run build; \
	fi

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
	rm -rf webkit/lib/*.framework

# Show help
help:
	@echo "term - AI-native terminal emulator"
	@echo ""
	@echo "Commands:"
	@echo "  make build          - Build the terminal binary"
	@echo "  make run            - Build and run"
	@echo "  make frontend       - Build frontend (xterm.js)"
	@echo "  make webkit-download - Download prebuilt WebKit"
	@echo "  make webkit-build   - Trigger WebKit build on CI"
	@echo "  make clean          - Clean build artifacts"
