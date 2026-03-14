# Contributing

We welcome both issue reports and pull requests! Please follow these guidelines to help maintainers respond effectively.

## Issues

### Before opening a new issue

- Use the search tool to check for existing issues or feature requests.
- Review existing issues and provide feedback or react to them.
- Use English for all communications.
- For questions or configuration problems, please use [GitHub Discussions](https://github.com/Mukbeast4/go-mt5/discussions).
- For security vulnerabilities, see [SECURITY.md](SECURITY.md) instead of opening a public issue.

### Reporting a bug

- Provide a clear description and a minimal reproducible code example.
- Include the Go version, OS, and go-mt5 version.
- Indicate whether you can reproduce the bug and describe steps to do so.

### Requesting a feature

- Describe the use case and why the feature is needed.
- If possible, include a proposed API showing how you'd use the feature.

## Pull Requests

### Before you start

1. Check for existing issues or PRs related to your change.
2. Open an issue first if the change is significant, so we can discuss the approach.

### Development workflow

1. Fork and clone the repository.
2. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/your-feature
   ```
3. Make your changes.
4. Run all checks:
   ```bash
   go fmt ./...
   go vet ./...
   go test -race ./...
   ```
5. Commit with a clear message following [Conventional Commits](https://www.conventionalcommits.org/):
   ```
   feat(trading): add trailing stop support
   fix(protocol): handle fragmented pipe reads
   ```
6. Push and open a pull request against `main`.

### Code guidelines

- Follow standard Go conventions (`gofmt`, `go vet`).
- Write tests for new features and bug fixes.
- Keep changes focused: one feature or fix per PR.
- No dead code, no commented-out code.
- Code should be self-explanatory; avoid unnecessary comments.

### Review process

- All PRs require at least one review before merge.
- CI must pass (tests, vet, format check).
- PRs are squash-merged into `main`.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.
