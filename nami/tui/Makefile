BINARY_NAME := nami
ENGINE_NAME := nami-engine
VERSION     := 0.1.0
BUILD_DIR   := release
GO_SRC      := ../cmd/nami-engine
PREFIX      ?= /usr/local/bin
WINDOWS_WRAPPER := bin/nami.cmd

UNIX_PLATFORMS := darwin-arm64 darwin-amd64 linux-amd64 linux-arm64
WINDOWS_PLATFORMS := windows-amd64 windows-arm64
PLATFORMS := $(UNIX_PLATFORMS) $(WINDOWS_PLATFORMS)

.PHONY: all clean release build-tui-launcher $(PLATFORMS)

all: release-local


# Build for current platform only (fast, for development)
release-local: clean build-tui-launcher
	@mkdir -p $(BUILD_DIR)
	GOOS=$$(go env GOOS) GOARCH=$$(go env GOARCH) go build -o $(BUILD_DIR)/$(ENGINE_NAME) $(GO_SRC)
	@echo ""
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME) + $(BUILD_DIR)/$(ENGINE_NAME)"
	@echo "The local launcher is a Bun script bundle, so Bun must be installed where it runs."
	@echo "Copy both to a directory in your PATH to install."

# Build a portable Bun launcher script for local installs.
build-tui-launcher:
	@mkdir -p $(BUILD_DIR)
	bun run build
	bun build --target=bun --outfile=$(BUILD_DIR)/$(BINARY_NAME) \
		bin/nami.js
	chmod +x $(BUILD_DIR)/$(BINARY_NAME)

# Build release assets for all platforms.
release: clean build-tui-launcher
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d- -f1); \
		arch=$$(echo $$platform | cut -d- -f2); \
		dir=$(BUILD_DIR)/$(BINARY_NAME)-$$platform; \
		asset_dir=$(BUILD_DIR)/assets; \
		binary_asset=$$asset_dir/$(BINARY_NAME)-$$platform; \
		echo "Building $$platform..."; \
		mkdir -p $$dir $$asset_dir; \
		install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $$dir/$(BINARY_NAME); \
		if [ "$$os" = "windows" ]; then \
			engine_name=$(ENGINE_NAME).exe; \
			wrapper_asset=$$asset_dir/$(BINARY_NAME)-$$platform.cmd; \
			engine_asset=$$asset_dir/$(ENGINE_NAME)-$$platform.exe; \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -o $$dir/$$engine_name $(GO_SRC); \
			install -m 644 $(WINDOWS_WRAPPER) $$dir/$(BINARY_NAME).cmd; \
			install -m 755 $$dir/$(BINARY_NAME) $$binary_asset; \
			install -m 644 $$dir/$(BINARY_NAME).cmd $$wrapper_asset; \
			install -m 755 $$dir/$$engine_name $$engine_asset; \
			(cd $(BUILD_DIR) && zip -qr $(BINARY_NAME)-$$platform.zip $(BINARY_NAME)-$$platform); \
			echo "  -> $$binary_asset"; \
			echo "  -> $$wrapper_asset"; \
			echo "  -> $$engine_asset"; \
			echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-$$platform.zip"; \
		else \
			engine_asset=$$asset_dir/$(ENGINE_NAME)-$$platform; \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -o $$dir/$(ENGINE_NAME) $(GO_SRC); \
			install -m 755 $$dir/$(BINARY_NAME) $$binary_asset; \
			install -m 755 $$dir/$(ENGINE_NAME) $$engine_asset; \
			tar -czf $(BUILD_DIR)/$(BINARY_NAME)-$$platform.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-$$platform; \
			echo "  -> $$binary_asset"; \
			echo "  -> $$engine_asset"; \
			echo "  -> $(BUILD_DIR)/$(BINARY_NAME)-$$platform.tar.gz"; \
		fi; \
	done
	@echo ""
	@echo "Release artifacts in $(BUILD_DIR)/"

clean:
	rm -rf $(BUILD_DIR)

# Install locally (copies to /usr/local/bin)
install: release-local
	install -d $(PREFIX)
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(PREFIX)/$(BINARY_NAME)
	install -m 755 $(BUILD_DIR)/$(ENGINE_NAME) $(PREFIX)/$(ENGINE_NAME)
	@echo "Installed to $(PREFIX)/"
