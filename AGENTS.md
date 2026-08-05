# Gog CLI

This repository contains the Google Workspace CLI used by ReBattery.

## Structure

- `cmd/gog` is the CLI entry point.
- `internal` contains commands, APIs, authentication, configuration, and output code.
- Keep tests next to the code. Integration tests belong in `internal/integration`.
- `docs` contains product and release details. `scripts` contains release helpers.

## Rules

- Keep JSON and plain output parseable on stdout. Send progress and hints to stderr.
- Treat Gmail label IDs as opaque and case-sensitive. Case-fold label names only during lookup.
- Keep credentials and tokens out of Git.
- Prefer the operating system keychain. Use the file keyring only in headless environments.
- Keep changes focused. Do not include unrelated refactors.

## Verification

- Run `make fmt`, `make lint`, and `make test` for normal changes.
- Run `make ci` for broad or release-sensitive changes.
- Run integration tests only when the change needs live Google API verification.

When Ari provides a PR for review, inspect it without changing branches. Land or merge it only when Ari asks.
