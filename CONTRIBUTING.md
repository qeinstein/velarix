# Contributing to Velarix

Thank you for your interest in contributing to the Velarix Epistemic Protocol!

## Development Guidelines

### Go Core (`/core`, `/api`)
- Use idiomatic Go formatting (`go fmt`).
- Ensure all core logic changes include unit tests. We strive for >80% test coverage in the `core` package.
- If you are modifying the Dominator Tree logic, please explain the complexity impacts in your PR.

### SDKs (`/sdks`)
- **Python:** Use `black` and `isort` for formatting. Ensure type hints are accurate.

### Pull Request Process
1. Fork the repository.
2. Create a feature branch (`git checkout -b feature/your-feature-name`).
3. Commit your changes (`git commit -m 'Add some feature'`).
4. Push to the branch (`git push origin feature/your-feature-name`).
5. Open a Pull Request on GitHub.

## Reporting Issues
Please use the GitHub Issue templates for bug reports and feature requests. Include reproduction steps whenever possible!
