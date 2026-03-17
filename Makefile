.PHONY: fmt vet tidy test lint install-hooks

fmt:
	@echo '> gofmt'
	@test -z "$$(gofmt -l .)" || { gofmt -l . && echo 'Run: gofmt -w .' && exit 1; }

vet:
	@echo '> go vet'
	go vet ./...

tidy:
	@echo '> go mod tidy'
	@cp go.mod go.mod.bak
	@[ -f go.sum ] && cp go.sum go.sum.bak || true
	@go mod tidy
	@diff -q go.mod go.mod.bak >/dev/null 2>&1 || { mv go.mod.bak go.mod; [ -f go.sum.bak ] && mv go.sum.bak go.sum || true; echo 'go.mod/go.sum are out of date. Run: go mod tidy'; exit 1; }
	@rm -f go.mod.bak go.sum.bak

test:
	@echo '> go test'
	go test ./...

lint: fmt vet tidy test
	@echo '=== all checks passed ==='

install-hooks:
	git config core.hooksPath .githooks
	@echo 'Git hooks installed (.githooks)'
