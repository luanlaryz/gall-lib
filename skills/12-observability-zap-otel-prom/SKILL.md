---
name: observability-zap-otel-prom
description: Instrument services with zap JSON logging, OpenTelemetry traces, Prometheus metrics, and Grafana dashboards including correlation IDs.
---

# Observability (zap + OTel + Prom)
Goal: provide consistent insight across logs, traces, and metrics with correlation IDs for every seller interaction.

## When to Use
- Adding/changing logging, tracing, metrics, or Grafana dashboards.
- Introducing new components (HTTP handler, worker, stream) needing instrumentation.
- Investigating incidents needing better observability.

## Non-negotiables
1. Logs emitted via zap in JSON format with required fields (see resource).
2. Traces instrumented using OpenTelemetry SDK exporting to collector defined in config.
3. Metrics exported via Prometheus `/metrics` endpoint with cataloged names.
4. Every log/metric includes `request_id` or `command_id` + `seller_id` when available.
5. Grafana dashboards updated when new metrics added.

## Do / Don't
- **Do** wrap contexts with `otel.Tracer` spans at boundaries (HTTP/gRPC/worker).
- **Do** log at `info` for successful checkpoints, `warn` for recoverable errors, `error` for failures.
- **Do** add exemplars to histograms when supported.
- **Don't** log sensitive payloads (PII, secrets); mask before logging.
- **Don't** create ad-hoc metric names; follow catalog.
- **Don't** emit logs inside hot loops unless at debug level (and guard with flag).

## Interfaces / Contracts
- Required log fields in [log_fields.md](resources/log_fields.md).
- Metrics list in [metrics_catalog.md](resources/metrics_catalog.md).
- Standard middleware wiring:
  ```go
  logger := zap.L()
  tracer := otel.Tracer("api")
  meter := global.Meter("api")
  ```

## Checklists
**Before coding**
- [ ] Decide which spans/metrics/logs are necessary.
- [ ] Confirm labels/tags align with catalog.
- [ ] Ensure config skill exposes endpoints/keys.

**During**
- [ ] Add context-aware logging (`logger.With(zap.String("seller_id", sellerID))`).
- [ ] Wrap operations with spans/histograms.
- [ ] Emit Prom metrics with `promauto` or manual registration.

**After**
- [ ] Run `make telemetry-check` (if available) or `go test` for instrumentation packages.
- [ ] Verify data showing up locally (OTel collector + Prom scrape).
- [ ] Update Grafana dashboards or snapshots.

## Definition of Done
- Logs/traces/metrics confirm new path and share IDs for correlation.
- Dashboards updated with panels for new metrics.
- Alerts configured/adjusted if thresholds changed.
- Documentation/Runbooks mention instrumentation.

## Minimal Examples
- Add HTTP span: `ctx, span := tracer.Start(ctx, "CreateCommand")` -> `defer span.End()` -> record attributes.
- Metric use: `commandsEnqueued.WithLabelValues(sellerID, commandType).Inc()` after enqueue.
