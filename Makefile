.PHONY: lint fmt lint-fix check build

CARGO ?= cargo
RUST_WORKSPACE_FLAGS ?= --workspace --all-targets
CLIPPY_FIX_FLAGS ?= --fix --allow-dirty --allow-staged

lint: fmt lint-fix build

fmt:
	$(CARGO) fmt --all

lint-fix:
	$(CARGO) clippy $(RUST_WORKSPACE_FLAGS) $(CLIPPY_FIX_FLAGS) -- -D warnings

check:
	$(CARGO) check $(RUST_WORKSPACE_FLAGS)

build:
	$(CARGO) build $(RUST_WORKSPACE_FLAGS)
