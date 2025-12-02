# Contributing to Clotilde

Thanks for your interest in contributing! This guide will help you get set up and understand how to work on the project.

## Getting Started

### Prerequisites

- Go 1.25 or later
- Git

### Initial Setup

After cloning the repository, run:

```bash
make setup-hooks
```

This configures git hooks that automatically format your code and enforce code quality standards.

## Development Workflow

### Building

```bash
make build     # Build the clotilde binary to dist/clotilde
make install   # Install to ~/.local/bin
```

### Testing

```bash
make test           # Run all tests
make test-watch     # Run tests in watch mode (useful during development)
make coverage       # Generate coverage report
```

### Code Quality

```bash
make fmt    # Format code with gofmt and goimports
make lint   # Run linter (requires golangci-lint)
```

**Note:** Code formatting (`gofmt`) is automatically enforced by the pre-commit hook, so you don't need to remember to run `make fmt` before committing.

## Git Hooks

A pre-commit hook is automatically configured to run `gofmt` on any staged Go files. This ensures consistent code formatting across the project and prevents "formatting only" commits.

### Manual Hook Setup

If for some reason the hooks aren't working, you can manually set them up:

```bash
git config core.hooksPath .githooks
chmod +x .githooks/*
```

## Project Structure

See [CLAUDE.md](CLAUDE.md) for architecture details, session structure, and implementation patterns.

## Making Changes

1. Create a feature branch: `git checkout -b feature/your-feature`
2. Make your changes
3. Run tests: `make test`
4. Run linter: `make lint`
5. Code will be automatically formatted on commit
6. Push and create a pull request

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/) format with scopes when relevant:

- `feat(session): add new feature description`
- `fix(hook): fix bug description`
- `docs: update documentation`
- `test: add test coverage`
- `chore(deps): update dependencies`

See [CLAUDE.md](CLAUDE.md) in the project root for detailed commit guidelines.

## Testing

- Write tests alongside code changes
- Test both success and error cases
- Keep tests focused and independent
- Use descriptive test names

Run tests in watch mode during development:

```bash
make test-watch
```

## Need Help?

- Check [CLAUDE.md](CLAUDE.md) for architecture and implementation details
- Review [docs/ROADMAP.md](docs/ROADMAP.md) for roadmap and future ideas
- Open an issue if you have questions
