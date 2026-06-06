# leakspok

[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/ifood/leakspok)](https://goreportcard.com/report/github.com/ifood/leakspok)
[![Coverage](https://img.shields.io/badge/coverage-93%25-brightgreen.svg)]()
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

**leakspok** is a Go library, inspired by [pii](https://github.com/gen0cide/pii), designed to detect and anonymize Personally Identifiable Information (PII) and sensitive data in strings and byte slices. It helps developers ensure data privacy and compliance by identifying potential information leaks and providing flexible anonymization strategies.

## Features

### Detection Capabilities

Leakspok can detect various types of sensitive information:

- **Brazilian Identifiers**
  - CPF (with validation using check digits)
  - CNPJ (numeric and alphanumeric formats with validation)

- **Financial Information**
  - Credit Card numbers (Visa, MasterCard)
  - Bank Information (IBAN)

- **Contact Information**
  - Email addresses (with domain validation)
  - Phone numbers (international formats)
  - Phone numbers with extensions

- **Network Information**
  - IPv4 addresses (with validation)
  - IPv6 addresses
  - URLs and links

- **US Identifiers**
  - Social Security Numbers (SSN)
  - Street addresses, PO boxes, and ZIP codes

- **Other Identifiers**
  - UUIDs (including v3, v4, v5, and GUIDs)
  - Vehicle Identification Numbers (VIN)

### Anonymization Strategies

Leakspok provides two built-in strategies for anonymizing detected sensitive data:

1. **REDACT**: Replace the entire match with a placeholder string (e.g., `[REDACTED]`)
2. **MASK**: Replace the first N characters with a masking character (e.g., `****`)

### Advanced Features

- **Exception Handling**: Define exceptions to skip specific patterns
- **Custom Rules**: Create custom detection rules with configurable filters and anonymization strategies
- **Concurrent Processing**: Parallel token and rule evaluation for high-throughput workloads
- **Pattern Composition**: Combine patterns using logical operators (AND, OR, NOT, Threshold)
- **Tokenization**: Smart tokenization that handles various text formats
- **Result Caching**: In-memory and Redis/Valkey-backed caching of rule matching results
- **Optional DataDog Tracing**: APM tracing via runtime configuration (`NewDatadogTracer` + `SetGlobalTracer`)

## Quickstart


```go
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ifood/leakspok"
	"github.com/ifood/leakspok/analyzer"
)

func main() {
	rules := []analyzer.Rule{
		leakspok.DefaultEmailRule,
		leakspok.DefaultCreditCardRule,
	}
	logger := slog.Default()
	runner := analyzer.NewSerialRulesRuner(logger, nil)
	sa := analyzer.NewStringAnalyzer(logger, runner)

	result, details := sa.Anonymize(
		context.Background(),
		rules,
		"Contact: john@example.com, Card: 4242-4242-4242-4242",
	)

	fmt.Println("Has findings:", details.HasFindings)
	fmt.Println("Anonymized:", result)
	// Output: Has findings: true
	// Anonymized: Contact: <REDACTED>, Card: <REDACTED>
}
```

## Installation

```bash
go get github.com/ifood/leakspok
```

## API Overview

### Core Components

Leakspok is organized around three core abstractions:

- **ByteAnalyzer** — tokenizes input, evaluates rules against each token, and applies anonymization strategies. Designed for byte-slice workloads.
- **StringAnalyzer** — thin wrapper around ByteAnalyzer that operates on Go strings.
- **RuleRunner** — executes rules against tokens. Two implementations: `SerialRulesRunner` (sequential) and `ConcurrentRulesRunner` (parallel via goroutine pool).

### Basic Usage

The simplest path uses pre-built default rules with a serial runner:

```go
logger := slog.Default()
runner := analyzer.NewSerialRulesRuner(logger, nil)
sa := analyzer.NewStringAnalyzer(logger, runner)

result, details := sa.Anonymize(ctx, rules, input)
// details.HasFindings — true if PII was detected
// details.Detected — slice of Entity types that were found
```

### Concurrent Processing

For high-throughput workloads, use `MakeByteAnalyzer` with concurrency options:

```go
opts := analyzer.RunnerOptions{
	Concurrency: analyzer.ConcurrencyOptions{
		Enabled:                   true,
		ConcurrentRuleProcessing:  true,  // parallel rule evaluation
		ConcurrentTokenProcessing: true,  // parallel token dispatch
		RuleRunnerPoolSize:        10,
	},
}

ba, err := analyzer.MakeByteAnalyzer(logger, opts)
// Use ba.Anonymize(ctx, rules, &outputBuf, data)
```

### Custom Rules

```go
rule := analyzer.Rule{
	Name:        "custom_id",
	Description: "Match custom identifier format",
	Matcher:     pattern.NewPatternMatcher(pattern.EntityUUID, pattern.PatternFunc(yourMatchFunc)),
	Exceptions: []analyzer.Exception{
		{
			Reason:  "Skip known safe value",
			Matcher: pattern.NewPatternMatcher(pattern.EntityUUID, pattern.Equal([]byte("safe-value"))),
		},
	},
	Settings: analyzer.RuleSettings{
		Strategy: analyzer.MASK,
		Mask:     &analyzer.MaskSettings{MaskingChar: "#", MaxSize: 6},
	},
}
```

### Default Rules

Pre-configured rules are available in the root `leakspok` package:

| Variable | Entity | Strategy |
|----------|--------|----------|
| `DefaultCPFRule` | Brazilian CPF | REDACT |
| `DefaultCNPJRule` | Brazilian CNPJ | REDACT |
| `DefaultEmailRule` | Email address | REDACT |
| `DefaultIPRule` | IPv4/IPv6 address | REDACT |
| `DefaultCreditCardRule` | Credit card number | REDACT |

All default rules use `REDACT` strategy with `<REDACTED>` as the placeholder.

## Package Structure

```
leakspok/
├── analyzer/              # Core analysis engine
│   ├── byte_analyzer.go   # Byte slice analyzer
│   ├── string_analyzer.go # String wrapper for byte analyzer
│   ├── serial_runner.go   # Sequential rule runner
│   ├── concurrent_runner.go # Parallel rule runner
│   ├── tokenization.go    # Input tokenization
│   ├── rules.go           # Rule definitions and settings
│   ├── factory.go         # MakeByteAnalyzer / MakeStringAnalyzer
│   ├── pool.go            # Worker pool abstraction
│   └── cache/             # Rule matching cache backends
├── pattern/               # Pattern matching system
│   ├── pattern.go         # Pattern interfaces and composition
│   ├── matcher.go         # Pre-built pattern matchers (15+ matchers)
│   ├── regex.go           # Match functions (regex and pure Go)
│   ├── entity.go          # Entity type constants
│   └── meta.go            # Metadata for patterns
├── monitoring/            # Observability
│   ├── pattern.go         # Logging support via slog
│   ├── cache.go           # Cache tracing
│   ├── tracing.go         # Tracer interface, GlobalTracer(), SetGlobalTracer()
│   └── tracing_datadog.go # DataDog tracer (NewDatadogTracer)
└── root package           # Default rules and convenience exports
```

## Benchmarks

Benchmarks run on Apple M4, Go 1.25:

| Benchmark | Operations/sec | Time/op | Bytes/op | Allocs/op |
|-----------|---------------|---------|----------|-----------|
| ByteAnalyzer.Anonymize | 34,935 | 32,575 ns | 31,945 | 891 |
| ByteAnalyzer.Anonymize (large input) | 5,310 | 215,905 ns | 199,346 | 5,691 |
| TokenIterator | 580,020 | 2,081 ns | 736 | 12 |
| EmailMatcher | 22,211,720 | 52.8 ns | 0 | 0 |
| CPFMatcher | 5,426,433 | 220 ns | 80 | 5 |
| CNPJMatcher | 2,391,625 | 498 ns | 640 | 14 |
| CreditCardMatcher | 1,000,000 | 1,076 ns | 176 | 11 |
| IPMatcher | 942,981 | 1,263 ns | 1,376 | 32 |

Run benchmarks locally:

```bash
go test -bench=. -benchmem ./...
```

## Supported Entity Types

| Entity | Constant | Detection |
|--------|----------|-----------|
| Email | `EntityEmail` | Email addresses with domain validation |
| CPF | `EntityCPF` | Brazilian CPF with check digit validation |
| CNPJ | `EntityCNPJ` | Brazilian CNPJ (numeric and alphanumeric) |
| IP Address | `EntityIPAddress` | IPv4 and IPv6 with validation |
| Credit Card | `EntityCreditCard` | Visa and MasterCard with Luhn check |
| Phone | `EntityPhone` | International phone numbers |
| URL / Link | `EntityLink` | URLs (excluding emails) |
| SSN | `EntitySSN` | US Social Security Numbers |
| Address | `EntityAddress` | Street addresses, PO boxes, ZIP codes |
| Bank Info | `EntityBankInfo` | IBAN and routing numbers |
| UUID | `EntityUUID` | UUID v3, v4, v5, and GUIDs |
| VIN | `EntityVIN` | Vehicle Identification Numbers |

## Observability

Leakspok includes built-in observability features:

- **Structured Logging**: Integration with `log/slog` for detailed pattern matching logs
- **Distributed Tracing**: Optional DataDog APM integration via `monitoring.NewDatadogTracer()`
- **Work IDs**: Automatic UUID generation for tracking analysis workflows

DataDog tracing is **disabled by default**. To enable it, set the global tracer at program startup:

```go
import "github.com/ifood/leakspok/monitoring"

func init() {
    monitoring.SetGlobalTracer(monitoring.NewDatadogTracer())
}
```

Without calling `SetGlobalTracer`, all tracing calls are no-ops with zero allocation overhead. The global tracer is concurrent-safe.

Logs include:
- Pattern matching start/end events
- Match results (true/false)
- Work ID for correlation
- Pattern names for debugging

## Testing

Run tests with:

```bash
# All tests with race detector and coverage
go test -race -count=1 -cover ./...

# Benchmarks
go test -bench=. -benchmem ./...

# With DataDog tracing enabled
monitoring.SetGlobalTracer(monitoring.NewDatadogTracer())
go test -race -count=1 ./...
```

## Contributing

1. Fork the repository on GitHub.
2. Clone the forked repository to your machine.
3. Create a new branch.
4. Make your changes and write tests when practical.
5. Commit changes to the branch.
6. Push changes to your fork.
7. Open a pull request.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for detailed version history and migration notes.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
