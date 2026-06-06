# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**leakspok** is a Go library for detecting and anonymizing Personally Identifiable Information (PII) and sensitive data in strings and byte slices. It provides flexible detection rules, composable pattern matchers, and multiple anonymization strategies for security and compliance use cases.

The library is designed to be:
- **Defensive**: Focused on identifying and removing sensitive data from logs and outputs
- **Flexible**: Supports custom rules, exceptions, and pattern composition
- **Observable**: Integrated with DataDog APM and structured logging
- **Performant**: Uses tokenization for accuracy and smart short-circuiting

## Common Development Commands

### Using Task Runner

This project uses [Task](https://taskfile.dev/) for command management. All commands are defined in `Taskfile.yml`.

| Command | Purpose |
|---------|---------|
| `task build` | Build the application binary |
| `task test` | Run all tests with race detector |
| `task test/unit` | Run unit tests only |
| `task test/e2e` | Run end-to-end tests (if available) |
| `task lint` | Run golangci-lint with strict configuration |
| `task format` | Format code with gofmt and goimports |
| `task gen` | Run go generate for mocks and codegen |
| `task vendor` | Vendor dependencies (tidy + vendor) |

### Running Single Tests

```bash
# Run a specific test function
go test -run TestFindCPF ./...

# Run tests in a single package
go test ./analyzer -v

# Run with coverage
go test -cover -coverprofile=coverage.out ./...
```

## Architecture & Package Structure

### Core Packages

#### `analyzer/` - Detection & Anonymization Engine
- **ByteAnalyzer**: Core analyzer that operates on byte slices
  - `Anonymize()`: Removes/masks sensitive data using configured strategies
- **StringAnalyzer**: Wrapper around ByteAnalyzer for string input
- **Rule Definition**: Defines matching rules with exceptions and anonymization settings
- **Mocks**: Auto-generated mocks in `analyzer/mocks/` for testing

Key types:
- `Rule`: Contains name, matcher, exceptions, and anonymization settings
- `Matcher`: Interface for pattern matching (must implement `Entity()` and `Match()`)
- `Exception`: Allows whitelisting specific patterns from rules
- `RuleSettings`: Controls anonymization strategy (REDACT or MASK)

#### `pattern/` - Pattern Matching System
Provides composable pattern matchers with logical operations:

- **Base Patterns**: Pre-built matchers in `matcher.go`
  - `EmailMatcher()`, `CPFMatcher()`, `CNPJMatcher()`
  - `CreditCardMatcher()`, `IPMatcher()`, `PhoneMatcher()`
  - `URLMatcher()`, `SSNMatcher()`, `UUIDMatcher()`

- **Pattern Composition** (`pattern.go`):
  - `And(patterns...)`: All patterns must match
  - `Or(patterns...)` / `Any(patterns...)`: At least one must match
  - `Not(pattern)`: Pattern must not match
  - `Threshold(count, patterns...)`: N-of-M must match

- **Pattern Interface**:
  - `Name()`: Returns pattern identifier
  - `Match(ctx, input)`: Returns true if input matches
  - Supports context for cancellation and observability

#### `monitoring/` - Observability
- **Logging**: Integration with `slog` for structured logging
- **Tracing**: DataDog APM integration via `github.com/DataDog/dd-trace-go/v2`
- **Context Integration**: Adds Work IDs and logger context for request tracing
- **Global Tracer**: Concurrent-safe `GlobalTracer()` / `SetGlobalTracer()` backed by `sync.RWMutex`
- **Datadog Setup**: `monitoring.NewDatadogTracer()` returns a `Tracer` that users set via `SetGlobalTracer()`

#### Root Package - Default Rules
- Provides pre-configured default rules in `rules.go`
- Default rules: CPF, CNPJ, Email, IP, CreditCard
- All use REDACT strategy with `<REDACTED>` placeholder

### Data Flow

```
Input (string/bytes)
  ↓
Tokenization (split by delimiters, remove escapes)
  ↓
For each Rule:
  - Check exceptions first (short-circuit if matched)
  - For each token: Match against rule.Matcher
  - If matched and not excepted:
    - Apply anonymization strategy (REDACT or MASK)
    - Track detected entities
  ↓
Output (anonymized string/bytes + metadata)
```

## Key Implementation Details

### Tokenization (`byte_analyzer.go`)
- **Purpose**: Splits input on delimiters and removes escape sequences
- **Delimiters**: Space, punctuation, special chars: `,.;!?()[]{}"/\'`
- **Important**: Tokens are matched individually, not the full input
- **Edge Case**: A token like `john.doe@example.com` may be processed with and without punctuation

### Anonymization Strategies
- **REDACT**: Replace entire match with placeholder (e.g., `[REDACTED]`)
- **MASK**: Replace first N characters with masking character (e.g., `****...`)

### Pattern Matching
- All pattern matchers accept `context.Context` for:
  - Cancellation
  - DataDog distributed tracing
  - Structured logging of pattern execution
- Pattern composition allows building complex detection rules with AND/OR/NOT logic

## Testing Strategy

- **Unit Tests**: Use `testing` package with table-driven tests
- **Mocks**: Auto-generated via `go.uber.org/mock/mockgen` (run `task gen`)
- **Test Files**: Tests in root package (e.g., `byte_analyzer_test.go`, `string_analyzer_test.go`)
- **Coverage**: Aim for comprehensive coverage; use `-race` flag to catch race conditions

## Code Quality

### Linting Configuration
- **Tool**: golangci-lint v1.64.8+ (very strict configuration in `.golangci.yml`)
- **Key Checks Enabled**:
  - errcheck: Unchecked errors
  - gosec: Security issues
  - govet: Suspicious constructs
  - gocognit: Cognitive complexity (max: 20)
  - gocyclo: Cyclomatic complexity
  - staticcheck: Go vet on steroids
  - gosimple: Code simplification opportunities

Run linting:
```bash
task lint
# or directly
golangci-lint run -c .golangci.yml
```

### Known Linter Suppressions
- `gochecknoglobals`: Default rules in `rules.go` are intentionally global; global tracer in `monitoring/tracing.go` uses `//nolint:gochecknoglobals` with mutex protection
- Some test files suppress security checks (gosec) as they're not security-critical

## Dependencies & Versioning

- **Go Version**: 1.24.0 (toolchain 1.24.3)
- **Key Dependencies**:
  - `code.ifoodcorp.com.br/ifood/security/libs/go/foodsec-go-sdk`: Internal logging and security utilities
  - `github.com/google/uuid`: UUID generation
  - `gopkg.in/DataDog/dd-trace-go.v2`: Observability
  - `github.com/stretchr/testify`: Testing assertions
  - `go.uber.org/mock`: Mock generation

Dependencies are vendored in `/vendor`. After updating:
```bash
task vendor  # This runs go mod tidy && go mod vendor
```

## CI/CD

### GitLab CI Pipeline (`.gitlab-ci.yml`)
- **lint**: Runs golangci-lint, reports code quality
- **unit_test**: Runs tests with coverage reports
- **release**: Automatic versioning and tagging based on `VERSION` file

### Local Pre-Commit Checks
- Configuration in `.pre-commit-config.yaml`

## Important Patterns & Idioms

### Creating Custom Rules
```go
rule := analyzer.Rule{
    Name:        "my_pattern",
    Description: "My custom pattern",
    Matcher:     pattern.PatternFunc(myMatchFunction),
    Exceptions: []analyzer.Exception{
        {
            Reason:  "Exception reason",
            Matcher: pattern.Equal("exception_value"),
        },
    },
    Settings: analyzer.RuleSettings{
        Strategy: analyzer.REDACT,
        Redact: &analyzer.RedactSettings{
            Placeholder: "[REDACTED]",
        },
    },
}
```

### Pattern Composition Example
```go
// Email not from internal domain
pattern := pattern.And(
    pattern.EmailMatcher(),
    pattern.Not(pattern.Regex("@company\\.com$")),
)
```

### Using ByteAnalyzer
```go
import (
	"bytes"
	"context"
	"log/slog"
	"github.com/New-Horizons-Team/leakspok"
	"github.com/New-Horizons-Team/leakspok/analyzer"
)

func main() {
	rules := []analyzer.Rule{leakspok.DefaultEmailRule}
	logger := slog.Default()
	ruleRunner := analyzer.NewSerialRulesRunner()
	ba := analyzer.NewByteAnalyzer(logger, ruleRunner)

	// Anonymize and write to buffer
	ctx := context.Background()
	var output bytes.Buffer
	details := ba.Anonymize(ctx, rules, &output, []byte("Contact: user@example.com"))

	fmt.Println("Has findings:", details.HasFindings)
	fmt.Println("Anonymized:", output.String())
}
```

## Defensive Security Notes

This library is designed for **defensive security**:
- It detects and anonymizes sensitive data in logs and outputs
- NOT meant for extracting or harvesting credentials
- Ideal for compliance (GDPR, LGPD) and security logging
- Exception handling allows whitelisting known safe values (e.g., support@company.com)

## Monitoring & Observability

### Structured Logging
- Uses `log/slog` for structured logging
- Pattern matching is automatically logged with:
  - Pattern name
  - Match result
  - Work ID for correlation
- Created in `monitoring/pattern.go`

### DataDog Integration
- Tracing is disabled by default (noop tracer)
- Enable at runtime: `monitoring.SetGlobalTracer(monitoring.NewDatadogTracer())`
- Global tracer is concurrent-safe (`sync.RWMutex` protected)
- No build tags required — DataDog tracer is always available via `NewDatadogTracer()`
- Work IDs for distributed tracing correlation

## Current Development Status

The project is on the `feat/detected-entities` branch, which appears to be working on:
- Entity type detection and tracking
- Anonymization details tracking (detected vs. anonymized entities)

Check git history for context on recent changes.

## Active Technologies
- Go 1.24.0 (toolchain 1.24.3) + `sync`, `sync/atomic`, `context`, `log/slog` (stdlib only — no new external deps) (feat/concurrent-runner)
- N/A (uses existing `CacheStore` interface) (feat/concurrent-runner)
- Go 1.24.0 (toolchain 1.24.3) + `sync`, `sync/atomic`, `context`, `log/slog`, `slices` (stdlib only — no new external deps) (feat/concurrent-runner)
- Go 1.24.0 (toolchain 1.24.3) + `github.com/panjf2000/ants/v2` (new), existing: `stretchr/testify`, `go.uber.org/mock`, `dd-trace-go.v1` (002-goroutine-pool)
- N/A (caching layer unchanged) (002-goroutine-pool)

## Recent Changes
- feat/concurrent-runner: Added Go 1.24.0 (toolchain 1.24.3) + `sync`, `sync/atomic`, `context`, `log/slog` (stdlib only — no new external deps)
