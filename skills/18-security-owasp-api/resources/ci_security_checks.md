# CI Security Checks (Makefile + GitHub Actions)

## Mandatory Gates
- `govulncheck` on module code.
- `gosec` scan (direct or through `golangci-lint`).
- Dependency hygiene: pinned versions and automated update review (Dependabot/Renovate if configured).

## Makefile Example
```makefile
.PHONY: security
security:
	GOCACHE=/tmp/go-build GOPATH=/tmp/go go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	gosec ./...

.PHONY: ci-checks
ci-checks: test lint security
```

If `gosec` runs through `golangci-lint`, keep:
```yaml
# .golangci.yml
linters:
  enable:
    - gosec
```

## GitHub Actions Example
```yaml
name: ci
on:
  pull_request:
  push:
    branches: [main]

jobs:
  checks:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make test
      - run: make lint
      - run: make security
```

## Dependency Hygiene
- Keep direct dependencies explicit in `go.mod`.
- Avoid floating tool versions in CI where reproducibility matters.
- If Dependabot is enabled, require PR review for security updates before merge.
