# Contributing to pgpipe

Thank you for your interest in contributing! pgpipe is a MySQL to PostgreSQL migration tool and welcomes all forms of contribution.

## Ways to contribute

- Report bugs via [GitHub Issues](https://github.com/RobertoGongora/pgpipe/issues)
- Request features via [GitHub Issues](https://github.com/RobertoGongora/pgpipe/issues)
- Submit pull requests for bug fixes or new features
- Improve documentation

## Development setup

**Prerequisites**
- Go 1.23 or later
- `golangci-lint` (for linting)
- Access to a MySQL instance and a PostgreSQL instance (for integration testing)

```bash
git clone https://github.com/RobertoGongora/pgpipe.git
cd pgpipe
make deps    # go mod tidy && go mod download
make build   # build the binary
make test    # run the test suite
```

## Project structure

See [AGENTS.md](AGENTS.md) for a detailed breakdown of every package, file, and architectural decision. It is the authoritative guide for contributors.

## Coding standards

Follow the conventions in [AGENTS.md § Code Style Guidelines](AGENTS.md#code-style-guidelines):

- **Imports**: three groups — stdlib / external / internal, separated by blank lines
- **Naming**: packages lowercase single-word; types PascalCase; interfaces end in `-er` or `-Interface`
- **Errors**: always wrap with context using `fmt.Errorf("failed to X: %w", err)`
- **Database operations**: always use `context.WithTimeout`; close resources with `defer`

Format code before committing:

```bash
make fmt
make lint
```

## Adding a new transform

The [AGENTS.md § Adding a New Transform](AGENTS.md#adding-a-new-transform) section has a step-by-step recipe.

## Testing

The project uses standard `go test`. Tests are organized as:

- `internal/testutil/` — shared mock clients and helpers
- `*_test.go` files alongside the code they test

Run the full suite:

```bash
make test       # verbose output
make coverage   # show per-package coverage
```

New code should include tests. Pure functions should have table-driven tests covering normal cases, edge cases, and error cases. See existing tests in `internal/db/types_test.go` for the preferred style.

For functions that require live database connections (`mysql.go`, `postgres.go`), use the mock clients from `internal/testutil/`.

## Pull request process

1. Fork the repository and create a branch: `git checkout -b feat/my-feature`
2. Make your changes with tests
3. Run `make fmt` and `make test` — both must pass
4. Update [CHANGELOG.md](CHANGELOG.md) under the `[Unreleased]` section
5. Update [AGENTS.md](AGENTS.md) if you changed the architecture
6. Open a pull request against `main`

Pull requests are reviewed on a best-effort basis. Small, focused PRs merge fastest.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating you agree to abide by its terms.

## Reporting security issues

Please do **not** file public issues for security vulnerabilities. See [SECURITY.md](SECURITY.md) for the responsible disclosure process.
