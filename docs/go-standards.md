# Go Engineering Standards

KiwiGuard sits in the security path for LLM applications. Backend changes should be simple, reviewable, observable, and safe under failure.

## Language and Tooling

- Use Go 1.25 or newer.
- Keep code formatted with `gofmt` and `goimports`.
- Run focused tests during development and `make verify` before review.
- Keep package APIs small and explicit.

## Package Design

- Keep domain logic independent from HTTP, SQL, CLI, and framework details.
- Put orchestration in application services or use cases.
- Keep adapters responsible for protocol or persistence translation.
- Keep composition in bootstrap packages.
- Avoid adding new cross-context dependencies unless there is a clear shared infrastructure reason.

## Error Handling

- Return errors instead of panicking in request, storage, and worker paths.
- Keep error strings lowercase and without trailing punctuation.
- Wrap propagated errors with `%w`.
- Preserve enough context for operators to identify the failing boundary.

## Concurrency

- Tie goroutines to `context.Context` cancellation where practical.
- Use bounded queues and explicit backpressure for production paths.
- Avoid unbounded goroutine creation in hot paths.
- Use `sync.WaitGroup.Go` when a wait group starts goroutines.

## Security and Privacy

- Do not log raw secrets.
- Do not store upstream API keys in configuration rows.
- Use synthetic prompts, responses, and credentials in tests.
- Treat request and response payload handling as privacy-sensitive.

## Testing

- Add tests for behavior changes.
- Prefer table-driven tests for policy, detector, routing, and configuration behavior.
- Include failure-path tests for storage, gateway, and event pipeline code.
- Keep benchmarks focused on hot path behavior.

## Comments

- Document exported Go symbols.
- Add comments when they explain intent, invariants, concurrency, privacy, or security behavior.
- Avoid comments that repeat the code.

## Verification

Useful commands:

```bash
make test-go
make test-go-race
make test-go-cover
make bench-go
make lint-go
make vuln-go
make tidy-check
make build-go
make verify
```
