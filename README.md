# leakspok

[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/Prosus-Cyber-Xchange/leakspok)](https://goreportcard.com/report/github.com/Prosus-Cyber-Xchange/leakspok)
[![Coverage](https://img.shields.io/badge/coverage-93%25-brightgreen.svg)]()
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

> An [iFood](https://ifood.com.br) open-source project by the AI Security team.

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

	"github.com/Prosus-Cyber-Xchange/leakspok"
	"github.com/Prosus-Cyber-Xchange/leakspok/analyzer"
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
go get github.com/Prosus-Cyber-Xchange/leakspok
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

## Supported Entity Types

| Entity | Constant | Entity ID | Detection |
|--------|----------|-----------|-----------|
| Email | `EntityEmail` | `EMAIL` | Email addresses with domain validation |
| CPF | `EntityCPF` | `CPF_NUMBER` | Brazilian CPF with check digit validation |
| CNPJ | `EntityCNPJ` | `CNPJ_NUMBER` | Brazilian CNPJ (numeric and alphanumeric) |
| IP Address | `EntityIPAddress` | `IP_ADDRESS` | IPv4 and IPv6 with validation |
| Credit Card | `EntityCreditCard` | `CREDIT_CARD` | Visa and MasterCard with Luhn check |
| Phone | `EntityPhone` | `PHONE` | International phone numbers |
| URL / Link | `EntityLink` | `LINK` | URLs (excluding emails) |
| SSN | `EntitySSN` | `SSN` | US Social Security Numbers |
| Address | `EntityAddress` | `ADDRESS` | Street addresses, PO boxes, ZIP codes |
| Bank Info | `EntityBankInfo` | `BANK_INFO` | IBAN and routing numbers |
| UUID | `EntityUUID` | `UUID` | UUID v3, v4, v5, and GUIDs |
| VIN | `EntityVIN` | `VIN` | Vehicle Identification Numbers |

## Observability

Leakspok includes built-in observability features:

- **Structured Logging**: Integration with `log/slog` for detailed pattern matching logs
- **Distributed Tracing**: Optional DataDog APM integration via `monitoring.NewDatadogTracer()`
- **Work IDs**: Automatic UUID generation for tracking analysis workflows

DataDog tracing is **disabled by default**. To enable it, set the global tracer at program startup:

```go
import "github.com/Prosus-Cyber-Xchange/leakspok/monitoring"

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

## Contact

This project is maintained by iFood's AI Security team:

| Name | Email |
|------|-------|
| Caio Cavalcante | caio.cavalcante@ifood.com.br |
| Emanuel Valente | emanuel.valente@ifood.com.br |
| José Almas | jose.almas@ifood.com.br |
| Michelle Mesquita | michelle.mesquita@ifood.com.br |

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
