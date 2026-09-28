VERSION := $(strip $(shell cat VERSION))
GO ?= go
RELEASE_DIR := .audit/release/$(VERSION)
RELEASE_PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64
BUILD_FLAGS := -trimpath -buildvcs=false -ldflags '-s -w -X kamaji/cmd.Version=$(VERSION)'

.PHONY: build release-assets

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
