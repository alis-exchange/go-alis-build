# golangci-lint works on one module at a time, so these targets run it in every
# module of the repo, each picking up the shared .golangci.yml at the root.
# Limit to some modules with, for example: make lint MODULES="./alog ./trace"
MODULES ?= $(shell find . -name go.mod -not -path './.git/*' -exec dirname {} \; | sort)

.PHONY: lint lint-fix fmt

# Report issues in every module; exits non-zero if any module has issues.
lint:
	@status=0; for m in $(MODULES); do echo "==> $$m"; (cd $$m && golangci-lint run ./...) || status=1; done; exit $$status

# Apply the fixes linters can make themselves (godot, perfsprint, intrange, ...).
lint-fix:
	@for m in $(MODULES); do echo "==> $$m"; (cd $$m && golangci-lint run --fix ./...) || true; done

# Format with the configured formatters (gofumpt, gci, golines).
fmt:
	@for m in $(MODULES); do echo "==> $$m"; (cd $$m && golangci-lint fmt ./...); done
