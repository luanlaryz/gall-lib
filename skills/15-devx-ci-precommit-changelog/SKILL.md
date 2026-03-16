---
name: devx-ci-precommit-changelog
description: Maintain developer experience: docker-compose stack, Makefile targets, GitHub Actions CI, pre-commit hooks, conventional commits, changelog automation.
---

# DevX, CI & Release Hygiene
Goal: keep local dev setup, CI, and release process smooth so every engineer/agent can ship confidently.

## When to Use
- Editing Makefile, docker-compose, CI workflows, or pre-commit configs.
- Adding dependencies to local stack (postgres/redis/localstack/otel/prom/grafana).
- Working on release automation, changelog, or commit conventions.

## Non-negotiables
1. Make targets documented and kept in sync with automation (see resource).
2. Local stack uses docker-compose with services: Postgres, Redis, LocalStack, OTel Collector, Prometheus, Grafana.
3. Pre-commit hooks installed and passing before pushing.
4. GitHub Actions workflows kept green; updates include workflow docs.
5. Conventional commits enforced plus changelog updates via git-cliff.

## Do / Don't
- **Do** use `.env` files for local credentials; exclude from git.
- **Do** provide troubleshooting steps in PRs when dev-env changes.
- **Do** ensure CI secrets referenced via GitHub env vars, not plaintext.
- **Don't** break backward compatibility for make targets; if renaming, provide shim.
- **Don't** skip updating docs when docker services change.
- **Don't** push directly to main; rely on PR with CI status.

## Interfaces / Contracts
- Make targets listed in [make_targets.md](resources/make_targets.md).
- CI pipeline & release process described in [ci_pipeline.md](resources/ci_pipeline.md).
- Pre-commit config expected under `.pre-commit-config.yaml` referencing hooks above.

## Checklists
**Before coding**
- [ ] Identify which developer workflow component changes.
- [ ] Confirm dependencies/services impacted.
- [ ] Notify teammates if downtime required.

**During**
- [ ] Update Makefile/docker-compose/CI config atomically.
- [ ] Test `make dev-up` + `make dev-down` locally.
- [ ] Run `pre-commit run --all-files` to ensure hook validity.

**After**
- [ ] Ensure GitHub Actions succeed on branch (use workflow_dispatch if needed).
- [ ] Update `CHANGELOG.md` via `make release` (dry run acceptable) for notable changes.
- [ ] Document new steps in README or skill resources as needed.

## Definition of Done
- Make + CI + pre-commit reflect latest commands.
- Local stack instructions verified and reproducible.
- Conventional commit message used; changelog updated if release-worthy.
- Automation scripts (git-cliff, commitlint) still functional.

## Minimal Examples
- Adding new service: update `docker-compose.yml`, extend `make dev-up` to depend on service, document env vars.
- Changing CI step: modify `.github/workflows/ci.yml`, run `act` or remote run, mention in PR.
