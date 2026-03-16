# Make Targets

| Target | Description |
|--------|-------------|
| `make bootstrap` | install go tools, pre-commit hooks |
| `make lint` | run golangci-lint, staticcheck |
| `make test` | run `go test ./...` with race + coverage |
| `make migrate MODULE=<module>` | run golang-migrate against local DB |
| `make dev-up` | docker compose up (postgres, redis, localstack, otel, prom, grafana) |
| `make dev-down` | stop compose stack |
| `make openapi-lint` | validate OpenAPI spec |
| `make buf` | run buf lint/generate |
| `make telemetry-check` | verify metrics endpoint + otel collector |
| `make release` | run changelog + tag (git-cliff + semver) |
