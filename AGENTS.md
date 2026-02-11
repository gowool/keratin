# AGENTS.md

This file provides guidance for agentic coding assistants working in this repository.

## Development Commands

### Testing
```bash
# Clear test cache
go clean -testcache

# Run all tests with race detection
go test -race ./...

# Run tests with verbose output
go test -race -v ./...

# Run specific test function
go test -race -run TestRoute_Use

# Run tests for specific file
go test -race -v router_test.go

# Run tests matching pattern
go test -race -run "TestRouter/.*Middleware"

# Run tests with coverage
go test -race -cover ./...

# Check coverage for specific file
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep router.go
```

### Building & Code Quality
```bash
# Build the package
go build ./...

# Format code (use go fmt)
go fmt ./...

# Run go vet
go vet ./...

# Clean up dependencies
go mod tidy

# Run static analysis (if available)
golangci-lint run -v --timeout=5m --build-tags=race --output.code-climate.path gl-code-quality-report.json
```

## Code Style Guidelines

### Imports
- Use standard library imports first, then third-party imports
- Group imports with blank lines between groups
- No unused imports - always run `go mod tidy` and `go vet`
- Import only what you need from the standard library

### Formatting
- Use `go fmt` for all code - no manual formatting
- Use tabs for indentation (Go standard)
- Maximum line length is not strictly enforced but keep it readable (~120 chars)
- Use `omitzero` JSON/YAML tags for zero-value omitempty behavior on time fields

### Types & Structs
- Define types using `type T baseType` pattern
- Exported types use PascalCase, unexported types use camelCase
- Pointer receiver methods for methods that modify state
- Value receiver methods for methods that don't modify state
- Function types implement interfaces (e.g., `HandlerFunc` implements `Handler`)

### Naming Conventions
- Structs: Nouns, PascalCase for exported
- Functions/Methods: PascalCase for exported, camelCase for unexported
- Variables: camelCase
- Private fields: camelCase, unexported (lowercase first letter)
- Package-level variables: PascalCase if exported, camelCase if not
- Errors: `ErrXxx` format (e.g., `ErrRouteNotFound`)
- Constants: PascalCase (e.g., `MIMEApplicationJSON`)

### Constructors
- Use `NewTypeName()` pattern for constructors
- Use `NewTypeName()` prefix for all factory functions
- Constructors should initialize default values and validate required parameters
- Panic with descriptive message for missing required parameters (e.g., `panic("router: error handler is required")`)

### Error Handling
- Define errors as package-level variables using `errors.New()`
- Use `errors.Is()` and `errors.AsType()` for error checking
- Wrap errors with context using `fmt.Errorf("msg: %w", err)`
- Return nil pointers for "not found" cases, don't wrap in error
- Use `fmt.Errorf` for errors with dynamic messages
- Custom error types should implement `Error() string` and `Unwrap() error` methods

### Interfaces
- Define interfaces with minimal necessary methods
- Use interface composition for combining related behaviors
- Implement interface compliance check: `var _ Interface = (*Concrete)(nil)`
- Keep interfaces small and focused (single responsibility)

### Fluent/Chaining API
- Methods that modify configuration should return `*Type` for method chaining
- Use chaining for route and group setup: `r.GET("/path", handler).UseFunc(mw)`
- Return `this` instead of `nil` for void-like methods to enable chaining

### Testing
- Use table-driven tests with `[]struct` pattern for comprehensive coverage
- Name test functions as `TestTypeName_MethodName` with descriptive test names
- Use `github.com/stretchr/testify/assert` for assertions
- Use `github.com/stretchr/testify/require` for fatal failures
- Create mock implementations using `github.com/stretchr/testify/mock`
- Mock files should be in `mock_test.go` file
- Use `go test -race` to catch data races
- Test both success and error paths
- Use `assert.Same(t, expected, actual)` to verify return values for chaining
- Use `t.Helper()` in helper functions to report correct line numbers
- Use `t.Parallel()` for independent tests to run concurrently
- Use `t.Cleanup()` for resource cleanup instead of deferred cleanup
- Use `t.TempDir()` for temporary files (automatically cleaned up)
- Test HTTP handlers using `net/http/httptest`
- Follow TDD: write failing test first, implement minimal code, then refactor
- Test behavior through public APIs, not private implementation details
- Avoid `time.Sleep()` in tests; use channels or conditions for synchronization

### Generics
- Use type constraints like `[T Resolver]` for generic types
- Keep generic type names concise (T, K, V)
- Document generic type constraints with comments

### Constants
- Define related constants together as a block
- Use `iota` for enumerations
- Group constants by logical category with comments

### String Building
- Use `strings.Builder` for concatenating multiple strings
- Use `fmt.Sprintf` for simple string formatting
- Avoid string concatenation with `+` in loops

### Maps & Slices
- Use `maps.Keys()` and `maps.Clone()` for map operations
- Initialize maps with `make()` when capacity is known

### Object Pooling
- Use `sync.Pool` for frequently allocated objects (e.g., response, error objects)
- Reset pooled objects before returning to pool
- Use pool in defer for proper cleanup

### Go 1.26 Features
- Use `iter.Seq[T]` for sequences when appropriate
- Leverage standard library functions from `maps`, `slices`, `iter` packages

### Visibility
- Export symbols only when needed by external packages
- Keep implementation details unexported
- Private fields lowercase, exported fields PascalCase

### Comments
- No inline comments for obvious code
- Document exported functions, types, and package behavior
- Keep comments concise and focused on "why" not "what"
