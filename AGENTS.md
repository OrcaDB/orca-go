# Go SDK — Agent Guidelines

Minimal Go client for the Orca/Lighthouse API. Zero external dependencies beyond the Go standard library.

## Running Commands

```bash
# From the go_sdk directory
go test ./...
go vet ./...
```

## Code Style

- Idiomatic Go: functional options, `context.Context` on all methods, `(value, error)` return pairs
- Single `Memory` type covers both labeled and scored memorysets (`Label`/`Score` are optional pointers)
- Use `*T` (pointer) for optional fields, never sentinel values
- Keep dependencies to the standard library only
- Tests use `net/http/httptest` for mocking the API

## Architecture

- `client.go` — HTTP client with auth, retries, env var config
- `types.go` — All shared data types (Memory, Filter, Prediction, Metadata)
- `errors.go` — `APIError` type and `IsNotFound`/`IsUnauthorized`/etc helpers
- `memoryset.go` — Memoryset operations (query, search, CRUD)
- `model.go` — Classification and regression model predictions
