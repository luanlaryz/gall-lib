# CI/CD Pipeline

## Pre-commit
- Hooks: `gofmt`, `golines`, `golangci-lint`, `go test ./...`, `markdownlint`, `commitlint` (conventional commits).
- Run `pre-commit install` after cloning.

## GitHub Actions
1. `ci.yml`
   - Steps: checkout -> setup Go -> cache -> `make lint` -> `make test` -> upload coverage.
2. `integration.yml`
   - Spins docker-compose (postgres, redis, localstack) -> runs integration tests -> publishes artifacts.
3. `release.yml`
   - Triggered on tags `v*` -> runs `git-cliff` to generate changelog -> builds docker images -> pushes to registry.

## Conventional Commits
- Format: `type(scope): subject`
- Types: feat, fix, chore, docs, refactor, test, ci, perf, build.
- Subject in imperative mood; max ~50 chars.

## Changelog
- `git-cliff` config under `.github/changelog.toml`.
- Run `make release` to update `CHANGELOG.md` automatically.
