# Repository Layout (Hexagonal)

```
/cmd/
  api/
    main.go              # wires Gin HTTP server, DI container, config loading
  worker/
    main.go              # wires worker loop, SQS consumer, Redis, pgx
/internal/
  domain/
    conversation/
      entity.go          # pure domain models (no infra imports)
      service.go         # business rules (ports only)
  application/
    usecase/
      send_message.go    # orchestrates domain + ports, transactional boundary
    port/
      repo.go            # interfaces consumed by use cases
      notifier.go
  adapter/
    postgres/
      conversation_repo.go  # pgx implementation of repo port
    redis/
      rate_limit_store.go
    aws/
      sqs_producer.go
    crm/
      talkdesk_client.go    # only place where CRM SDK appears
/pkg/
  config/
  logger/
  telemetry/

```

**Guidelines**
- Each adapter implements exactly one port interface; fan-in happens at application layer.
- Domain structs keep JSON tags off; adapters add DTOs for IO formats.
- Command binaries import only `/internal/{application,adapter,domain}` through a wiring package (e.g., `/internal/app`).
- Unit tests live next to code; integration/system tests live under `/test/`.
