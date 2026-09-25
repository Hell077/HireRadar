# Backend MVP implementation plan

Status: implementation in progress. Source: `apps/backend/agent.md`. This plan records the order of work and acceptance gates; a milestone is complete only when its full gate passes.

## Accepted decisions

- Keep the backend as a Go modular monolith with domain, application, and adapter boundaries. Domain code must not import transport or infrastructure packages; application use cases depend on ports.
- Use the existing Fiber + Huma HTTP stack and its OpenAPI contract. The `net/http` + chi choice in the architecture source is superseded by this decision. Keep transport details inside the HTTP adapter.
- Target Go 1.27.0 for backend builds. Consider Go 1.26 language additions (`new(expression)` and self-referential generic type parameter lists) and Go 1.27 additions (generic methods, nested field-selector keys in struct literals, and broader generic function inference) when they make an implementation clearer. Generic methods cannot satisfy interface methods that declare type parameters.
- PostgreSQL is the source of truth. Use pgx and goose for persistence and migrations; Redis supports queues, caching, and temporary state. Store resumes privately behind an S3-compatible storage port.
- Run API, worker, scheduler, and Telegram responsibilities as separately deployable executables that share domain and application packages. Start with one bounded worker implementation and split deployment capacity when needed.
- Store raw source payloads before normalization. Make background jobs idempotent and publish cross-module events through a transactional outbox.
- Use deterministic eligibility and matching for the MVP. AI and semantic ranking are later extensions; an unknown location must remain unknown, and `remote` does not imply worldwide eligibility.

## Delivery sequence

Each milestone is a reviewable change with its own migration, tests, documentation update, and working API contract where applicable. Do not create empty modules in advance.

| Milestone                 | Deliverable                                                                                                                                                                                                  | Completion gate                                                                                                                                     |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0. Foundation             | Configuration validation, structured logging and request IDs, graceful shutdown, `/health` and dependency-aware `/ready`; initial PostgreSQL, Redis, migration, and outbox wiring. Preserve Fiber + Huma.    | Local API starts with valid configuration, fails clearly on missing required secrets, and readiness reports unavailable required dependencies.      |
| 1. Identity               | User, auth, and session domains; registration, login, refresh rotation, logout, email verification, and password reset. Argon2id hashing, opaque hashed refresh tokens, transaction boundaries, rate limits. | Auth API and database integration tests cover duplicate email, token replay, expiration, and revocation.                                            |
| 2. Candidate profile      | Profile, normalized skills and aliases, desired positions, job preferences, and per-source preferences. Publish `ProfileChanged` through the outbox.                                                         | API tests cover ownership and validation; changed preferences produce one durable event.                                                            |
| 3. Resume                 | Private S3-compatible upload/download port, presigned upload completion, MIME/signature/size checks, parsing worker, and reviewable profile suggestions.                                                     | Upload and processing are idempotent; parsed suggestions never silently edit the profile.                                                           |
| 4. Source ingestion       | Source registry and cursor/run tracking, scheduler enqueue, bounded worker, Greenhouse/Lever/Ashby adapters, GitHub discovery, raw payload storage, retries and failure isolation.                           | Adapter contracts and integration tests prove stable external IDs, terminating pagination, independent source failures, and replayable raw records. |
| 5. Job catalog            | Company and job domains, location/eligibility classification, normalization, multi-source references, deduplication, lifecycle, cursor-based feed, and source monitoring APIs.                               | Tests cover Kazakhstan eligibility, unknown location, duplicate sources, official-source priority, and delayed closure after missing listings.      |
| 6. Matching               | Candidate preselection, hard filters, configurable rule weights, explanation, match persistence, and recent-job rematch on profile changes.                                                                  | Ineligible candidates never reach scoring or notifications; repeated events do not duplicate matches.                                               |
| 7. Telegram delivery      | Single-use linking token, verified webhook, notification preferences and quiet hours, outbox consumer, retry/dead-letter handling, and ownership-checked feedback actions.                                   | Tests cover token reuse, webhook authentication, callback ownership, quiet hours, and notification deduplication.                                   |
| 8. Operations and release | Saved/applied/hidden job flows, admin source monitoring and audit trail, metrics/traces, Docker/Railway commands, expand/contract migration procedure, and end-to-end pipeline checks.                       | A seeded source can be fetched, stored raw, normalized, matched, and delivered once; worker restart safely resumes processing.                      |

## Current implementation state

- Milestone 0 is complete: Go 1.27.0 module version, validated environment configuration, JSON logs and request IDs, graceful API shutdown, PostgreSQL and Redis clients, dependency-aware `/api/v1/ready`, and the first goose outbox migration are implemented and verified in Docker. The API applies migrations before it begins listening.
- Development can start without external services to expose Huma OpenAPI. In this mode readiness returns 503 and auth endpoints return 503. Production startup requires `DATABASE_URL`, `REDIS_URL`, and a base64-encoded 32-byte Ed25519 `JWT_PRIVATE_KEY`.
- The API applies migrations at startup while holding a PostgreSQL advisory lock. Use a Go 1.27.0 executable for local runs; the root client generator accepts `GO_BINARY` when the default `go` command points to another version.
- Docker verification covered API-run migration and idempotent restart, PostgreSQL/Redis readiness, RustFS bucket creation, Redis failure and recovery, and API shutdown. Milestone 1 is complete: registration, Argon2id hashing, Ed25519 access tokens, refresh rotation, logout, email verification, password reset, Redis-backed per-IP auth rate limits, and an SMTP outbox worker are implemented. The Docker stack routes email to Mailpit; the web registration, sign-in, verification, forgot-password, and reset-password pages submit to the API. Sign-in stores tokens in HTTP-only cookies. Automated PostgreSQL integration tests cover duplicate email, token expiration and replay, logout, and reset-triggered session revocation. Password reset revokes refresh sessions; previously issued 15-minute access tokens remain valid until expiry. Milestones 2–8 remain planned.
- Milestone 2 is in progress: migration 00003 adds profile, normalized skill, alias, desired-position, job-preference, and source-preference tables; migration 00004 seeds common skill aliases. Authenticated `GET` and `PUT` APIs cover profile fields, skills, and desired positions. Changed data emits `profile.changed` in the same PostgreSQL transaction; equivalent aliases and normalized position names are stable, and unchanged replacements emit no event. Job preferences and per-source preferences are next.
- The web application now calls every currently published identity and candidate-profile endpoint: registration, login, refresh rotation, logout, email verification, password reset, profile read/update, and skill read/replace. Resume, preference, vacancy, saved-job, notification, and Telegram UI remains presentation-only until its corresponding milestone publishes an API contract.
- The backend Dockerfile uses Go 1.27.0. The root `docker-compose.yml` runs the API, PostgreSQL, Redis, RustFS, and a one-shot AWS CLI bucket initializer. The S3 environment values are prepared for the resume adapter; the current API does not yet consume them.

## Local Docker stack

1. Copy `.env.docker.example` to `.env` and replace its placeholder credentials. The `.env` file is ignored by Git.
2. Run `docker compose up --build -d` at the repository root. PostgreSQL, Redis, and RustFS stay on the Compose network; the API is available at `http://127.0.0.1:8080`, RustFS exposes its S3 API and console on localhost ports 9000 and 9001, and Mailpit's local inbox is at `http://127.0.0.1:8025`.
3. Check `http://127.0.0.1:8080/api/v1/ready`; it returns 200 after the API applies migrations and PostgreSQL and Redis respond. The email worker starts after the API is healthy and does not run migrations. Use `docker compose logs api email-worker rustfs-init` to inspect startup or delivery failures. When Next.js is not on `http://localhost:3000`, set `APP_PUBLIC_URL` for email links; set `API_BASE_URL` for the Next.js server when the API is not on `http://127.0.0.1:8080`.
4. Run the PostgreSQL identity integration test against this stack with `docker run --rm --network hireradar_default --mount type=bind,src="$PWD/apps/backend",dst=/src,readonly -w /src -e PGPASSWORD="$POSTGRES_PASSWORD" -e TEST_DATABASE_URL='postgres://hireradar@postgres:5432/hireradar?sslmode=disable' golang:1.27.0-alpine3.24 go test ./internal/auth/adapters/postgres -run TestIdentityPostgresIntegration -count=1`.

## Data and dependency order

1. Introduce migrations alongside each aggregate, with database uniqueness for email, skill names, source/external references, user/job matches, saved jobs, and notification identity. Keep migrations compatible with rolling deployments.
2. Define ports around use cases, then implement PostgreSQL, Redis, S3, source, and Telegram adapters. Compose dependencies explicitly in each executable.
3. Persist the source cursor, sync run, raw payload, normalized job, and outbox event in deliberate transaction boundaries. A failed stage must be retryable without losing a raw job or duplicating a later effect.
4. Publish API changes through Huma's OpenAPI output and update generated web types in the same change as the endpoint.

## Verification per milestone

- Run Go formatting, `go vet ./...`, `go test ./...`, and `go build ./...` from `apps/backend`; add database and adapter integration tests when those systems are introduced.
- Run the relevant web type check/build after changing generated API contracts.
- Review migration up/down behavior and API contract changes before deployment. Test the full ingestion-to-notification path at milestone 8.

## Deferred after MVP

Keep pgvector and semantic ranking, AI analysis, additional ATS adapters, advanced source discovery, email/digest delivery, analytics, and automatic applications out of the MVP. Add them after the deterministic pipeline is working and measured.
