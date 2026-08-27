# Contributing to LLM Client Go

Thank you for your interest in contributing to LLM Client Go! We welcome bug reports, feature suggestions, documentation improvements, and code contributions.

---

## Code of Conduct

Please review and adhere to our [Code of Conduct](CODE_OF_CONDUCT.md) in all project interactions.

---

## Development Workflow: Test-Driven Development (TDD)

This project strictly follows **Test-Driven Development (TDD)**. All new features and bug fixes must follow the Red-Green-Refactor cycle:

1. **Red**: Write a failing test in the relevant `*_test.go` file demonstrating the expected behavior or bug. Run `task test` to confirm it fails on assertion.
2. **Green**: Write the minimal implementation required to pass the test.
3. **Refactor**: Clean up the code while keeping all tests passing.

For detailed guidelines, see [AGENTS.md](AGENTS.md).

---

## Prerequisites

- **Go**: 1.25 or higher
- **Task**: [taskfile.dev](https://taskfile.dev) (`go install github.com/go-task/task/v3/cmd/task@latest` or `brew install go-task`)

---

## Quickstart

```bash
# Clone the repository
git clone https://github.com/wkqco33/LLM_client_go.git
cd LLM_client_go

# Run tests
task test

# Watch mode for rapid TDD loops
task --watch test

# Run linter and formatter
task lint
task fmt
```

---

## Coding Conventions

- **Standard Library Assertions**: Use `if got != want { t.Errorf(...) }` rather than external assertion frameworks.
- **Error Handling**: Use `fmt.Errorf("package: stage: %w", err)` for actionable error wrapping.
- **Provider Retry Policy in Tests**: Use `RetryPolicy: noRetry` (`&retry.Policy{}`) for tests that trigger HTTP error status codes to avoid backoff delays.
- **No Leaked Secrets**: Never commit real API keys, credentials, or personal access tokens. Use placeholders like `"test-key"`, `"dummy-token"`, `"your_openai_api_key"`.
- **Formatting**: Format all code using `gofmt -s -w .` before submitting a pull request.

---

## Pull Request Checklist

Before submitting your pull request, please verify:

- [ ] `task test` passes with 0 failures.
- [ ] `task lint` (`go vet ./...`) reports 0 issues.
- [ ] Code is formatted with `task fmt` (`gofmt -s -w .`).
- [ ] New features or bug fixes include corresponding unit tests.
- [ ] Documentation (README, docs, docstrings) is updated where applicable.
