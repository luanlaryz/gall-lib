---
name: config-viper-flags
description: Manage configuration via Viper, load env-aware settings, and operate feature flags per seller.
---

# Config & Feature Flags
Goal: centralize configuration (Viper) and seller-specific feature flags without hardcoding per-environment differences.

## When to Use
- Changing config structure, adding options, or wiring Viper usage.
- Implementing feature flag evaluation or overrides.
- Handling secrets, env var parsing, or config-driven behavior.

## Non-negotiables
1. Viper loads defaults, config file, then env vars (highest priority).
2. Config objects passed via dependency injection; no global `viper.Get` outside setup.
3. Feature flags stored in Postgres + cached in Redis (60s TTL) with per-seller overrides.
4. Seller-level toggles evaluated in application layer before calling providers.
5. Secrets fetched from secure store (SSM/Doppler) not committed.

## Do / Don't
- **Do** create typed config structs covering all modules; avoid map[string]any.
- **Do** expose config via `pkg/config` package returning immutable copies.
- **Do** document new fields in [config_shape.md](resources/config_shape.md).
- **Don't** read env vars deep inside business logic.
- **Don't** use feature flags for sensitive security controls (deploy gating instead).
- **Don't** rely on default values silently; log config summary on startup.

## Interfaces / Contracts
- Config struct example:
  ```go
  type AppConfig struct {
      Env   string
      HTTP  HTTPConfig
      Redis RedisConfig
      Flags FlagConfig
  }
  ```
- Feature flag store: `flagStore.Enabled(ctx, sellerID, "streaming_default")`.
- Config shape documented in resource file.

## Checklists
**Before coding**
- [ ] Decide which module(s) need new config knobs.
- [ ] Define defaults + env overrides.
- [ ] Plan migration path for existing deployments.

**During**
- [ ] Update config structs + Viper bindings.
- [ ] Add validation logic (panic on missing required fields) during startup.
- [ ] Implement flag evaluation paths with caching + metrics.

**After**
- [ ] Update docs/resources + sample env files.
- [ ] Add tests ensuring config parsing works (use `t.Setenv`).
- [ ] Bump config version in release/changelog.

## Definition of Done
- Config struct + Viper wiring compile and pass tests.
- Feature flags accessible per seller with redis cache + fallback.
- Secrets handled securely.
- Documented shape + usage instructions.

## Minimal Examples
- Startup: `cfg := config.Load()` -> `server := app.NewServer(cfg)`.
- Flag check: `if !flagStore.Enabled(ctx, sellerID, "streaming_default") { disableStreaming() }`.
