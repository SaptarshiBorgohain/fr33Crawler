# Contributing

Thanks for your interest in contributing! This guide explains how to get set up and submit changes.

## Getting started
1. Fork the repo and create a feature branch.
2. Keep changes focused and add tests when you change behavior.
3. Run the relevant checks before opening a PR.

## Development setup
### Backend (Go)
- `cd backend`
- `go mod download`
- `go test ./tests/...`

### Python SDK
- `cd sdk/python`
- `pip install -e .[dev]`
- `pytest`

## Coding standards
- Prefer small, readable functions.
- Add tests for new features and bug fixes.
- Keep public APIs documented in the README.

## Submitting changes
- Open a PR with a clear description and rationale.
- Link any issues and include test output.

## Reporting issues
- Use GitHub Issues and include logs, steps to reproduce, and expected/actual behavior.
