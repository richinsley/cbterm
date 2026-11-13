.PHONY: all build clean run run-debug web dev frontend frontend-clean

BINARY := bin/cbterm

all: build

# Build everything (frontend + Go binary)
build: frontend
	@mkdir -p bin
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/cbterm
	@echo "Built $(BINARY)"

# Run the terminal (native webview window)
run: build
	./$(BINARY)

# Run in debug mode
run-debug: build
	./$(BINARY) --debug

# Run as a terminal server (prints a URL for any browser)
web: build
	./$(BINARY) --web

# Development mode: run frontend dev server + Go binary
dev:
	@echo "Starting development mode..."
	@echo "1. Run 'cd frontend && npm run dev' in another terminal"
	@echo "2. Then run: ./$(BINARY) --debug"

# Build frontend (xterm.js)
frontend:
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install --silent
	@echo "Building frontend..."
	@cd frontend && npm run build

# Clean frontend and rebuild
frontend-clean:
	rm -rf frontend/node_modules frontend/dist
	$(MAKE) frontend

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf frontend/dist/

# Full clean (including node_modules)
distclean: clean
	rm -rf frontend/node_modules

# Show help
help:
	@echo "cbterm - AI-native terminal emulator"
	@echo ""
	@echo "Commands:"
	@echo "  make build          - Build frontend and Go binary"
	@echo "  make run            - Build and run (native webview)"
	@echo "  make run-debug      - Build and run with debug logging"
	@echo "  make web            - Build and run as a terminal server"
	@echo "  make dev            - Instructions for development mode"
	@echo "  make frontend       - Build frontend only"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make distclean      - Clean everything including node_modules"
