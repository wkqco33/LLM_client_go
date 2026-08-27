## Description

Please include a summary of the change and which issue is fixed or feature implemented.

Fixes # (issue)

## Type of change

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Performance improvement
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## TDD & Quality Checklist

- [ ] I have followed the TDD workflow (Red -> Green -> Refactor) as described in [AGENTS.md](AGENTS.md).
- [ ] `task test` passes with 0 failures.
- [ ] `task lint` (`go vet ./...`) reports 0 issues.
- [ ] `task fmt` (`gofmt -s -w .`) was run to format all modified files.
- [ ] No sensitive credentials, API keys, or personal secrets are included.
- [ ] Documentation is updated accordingly.
