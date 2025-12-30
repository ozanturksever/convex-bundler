# Contributing to Convex Bundler

Thank you for your interest in contributing to Convex Bundler! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions. We welcome contributors of all experience levels.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Docker (for running integration tests)
- Git

### Setting Up Your Development Environment

1. Fork the repository on GitHub

2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/convex-bundler.git
   cd convex-bundler
   ```

3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/ozanturksever/convex-bundler.git
   ```

4. Install dependencies:
   ```bash
   go mod download
   ```

5. Build and test:
   ```bash
   make build
   make test
   ```

## Making Changes

### Branching Strategy

1. Create a new branch for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

2. Make your changes and commit them with clear, descriptive commit messages.

3. Keep your branch up to date with upstream:
   ```bash
   git fetch upstream
   git rebase upstream/master
   ```

### Coding Standards

- Follow standard Go conventions and idioms
- Run `go fmt` before committing
- Run `go vet` to catch common mistakes
- Write tests for new functionality
- Keep functions focused and small
- Add comments for exported functions and types

### Running Tests

```bash
# Run all tests
make test

# Run tests without integration tests (faster)
make test-short

# Run tests with coverage
make coverage
```

### Code Quality Checks

Before submitting a pull request, run all checks:

```bash
make check
```

This will run formatting, vetting, linting, and tests.

## Submitting Changes

### Pull Request Process

1. Ensure all tests pass locally
2. Update documentation if needed
3. Push your branch to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```
4. Create a Pull Request on GitHub
5. Fill in the PR template with relevant information
6. Wait for review and address any feedback

### Pull Request Guidelines

- **Title**: Use a clear, descriptive title
- **Description**: Explain what changes you made and why
- **Tests**: Include tests for new functionality
- **Documentation**: Update README.md or other docs if needed
- **Breaking Changes**: Clearly note any breaking changes

### Commit Messages

Follow conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `chore`: Maintenance tasks

Examples:
```
feat(cli): add --verbose flag for detailed output
fix(predeploy): handle Docker timeout gracefully
docs(readme): add installation instructions
```

## Reporting Issues

### Bug Reports

When reporting a bug, please include:

1. **Description**: Clear description of the bug
2. **Steps to Reproduce**: Minimal steps to reproduce the issue
3. **Expected Behavior**: What you expected to happen
4. **Actual Behavior**: What actually happened
5. **Environment**: Go version, OS, Docker version
6. **Logs**: Relevant error messages or logs

### Feature Requests

For feature requests, please include:

1. **Description**: Clear description of the feature
2. **Use Case**: Why this feature would be useful
3. **Proposed Solution**: How you think it could be implemented (optional)

## Project Structure

```
.
├── main.go                 # Main entry point
├── pkg/
│   ├── bundle/            # Bundle creation logic
│   ├── cli/               # CLI parsing
│   ├── credentials/       # Credential generation
│   ├── manifest/          # Manifest generation
│   ├── predeploy/         # Pre-deployment logic
│   └── version/           # Version detection
├── docker/
│   └── convex-predeploy/  # Docker image for pre-deployment
├── scripts/               # Utility scripts
└── testdata/              # Test fixtures
```

## Getting Help

If you need help or have questions:

1. Check existing issues and documentation
2. Open a new issue with your question
3. Be patient and respectful when waiting for responses

## License

By contributing to Convex Bundler, you agree that your contributions will be licensed under the Apache License 2.0.
