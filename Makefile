.PHONY: test

define MODFILE
module github.com/wiselike/revel-cron

go 1.17
endef
export MODFILE

test:
	@if [ ! -f go.mod ]; then \
		echo "$$MODFILE" > go.mod; \
	fi
	@- go mod tidy || true
	go test ./...
	@- \rm go.mod go.sum 2>/dev/null || true