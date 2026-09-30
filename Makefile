VERSION := $(strip $(shell cat VERSION))
GO ?= go
RELEASE_DIR := .audit/release/$(VERSION)
RELEASE_PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64
BUILD_FLAGS := -trimpath -buildvcs=false -ldflags '-s -w -X kamaji/cmd.Version=$(VERSION)'

.PHONY: build release-assets homebrew-formula

build:
	$(GO) build $(BUILD_FLAGS) -o kamaji .

release-assets:
	@set -eu; \
	mkdir -p '$(RELEASE_DIR)'; \
	for platform in $(RELEASE_PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		name=kamaji_$(VERSION)_$${os}_$${arch}; \
		stage='$(RELEASE_DIR)'/$$name; \
		mkdir -p "$$stage"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build $(BUILD_FLAGS) -o "$$stage/kamaji" .; \
		cp LICENSE "$$stage/LICENSE"; \
		COPYFILE_DISABLE=1 tar -czf "$(RELEASE_DIR)/$$name.tar.gz" -C "$$stage" kamaji LICENSE; \
	done
	cd '$(RELEASE_DIR)' && shasum -a 256 kamaji_$(VERSION)_darwin_amd64.tar.gz kamaji_$(VERSION)_darwin_arm64.tar.gz kamaji_$(VERSION)_linux_amd64.tar.gz kamaji_$(VERSION)_linux_arm64.tar.gz > SHA256SUMS

# Generate the tap formula from the exact archives that will be published.
# Run release-assets first; this target verifies rather than rebuilds them.
homebrew-formula:
	@set -eu; \
	(cd '$(RELEASE_DIR)' && shasum -a 256 -c SHA256SUMS); \
	mkdir -p '$(RELEASE_DIR)/homebrew/Formula'; \
	darwin_arm64=$$(shasum -a 256 '$(RELEASE_DIR)/kamaji_$(VERSION)_darwin_arm64.tar.gz' | cut -d ' ' -f 1); \
	darwin_amd64=$$(shasum -a 256 '$(RELEASE_DIR)/kamaji_$(VERSION)_darwin_amd64.tar.gz' | cut -d ' ' -f 1); \
	linux_arm64=$$(shasum -a 256 '$(RELEASE_DIR)/kamaji_$(VERSION)_linux_arm64.tar.gz' | cut -d ' ' -f 1); \
	linux_amd64=$$(shasum -a 256 '$(RELEASE_DIR)/kamaji_$(VERSION)_linux_amd64.tar.gz' | cut -d ' ' -f 1); \
	sed -e 's/@VERSION@/$(patsubst v%,%,$(VERSION))/g' \
	    -e "s/@DARWIN_ARM64_SHA256@/$$darwin_arm64/g" \
	    -e "s/@DARWIN_AMD64_SHA256@/$$darwin_amd64/g" \
	    -e "s/@LINUX_ARM64_SHA256@/$$linux_arm64/g" \
	    -e "s/@LINUX_AMD64_SHA256@/$$linux_amd64/g" \
	    homebrew/kamaji.rb.in > '$(RELEASE_DIR)/homebrew/Formula/kamaji.rb'
