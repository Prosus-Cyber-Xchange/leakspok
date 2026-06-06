// Concurrent example: demonstrates high-throughput PII processing with leakspok.
//
// This program shows three operating modes for the ByteAnalyzer:
//  1. Serial — baseline, single-threaded token+rule evaluation
//  2. ConcurrentTokenProcessing — each token dispatched to a goroutine pool
//  3. ConcurrentRuleProcessing — rules evaluated in parallel via ConcurrentRulesRunner
//
// The input is a synthetic log file (~3 KB) containing multiple PII types
// (CPF, CNPJ, email, credit card, IP addresses). Each mode processes the
// same input, and the program prints anonymization results with timing
// information so you can compare throughput.
//
// Run with:
//
//	go run ./examples/concurrent/
//
// Output shows the detection results for each mode and elapsed wall-clock time.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Prosus-Cyber-Xchange/leakspok"
	"github.com/Prosus-Cyber-Xchange/leakspok/analyzer"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
)

// largeInput builds a synthetic log with multiple PII types embedded in
// realistic log lines. The total size exceeds 1000 bytes and contains
// several dozen tokens so that concurrent processing is exercised.
func largeInput() []byte {
	var buf bytes.Buffer

	// PII values that survive tokenization (no '/' delimiter in the middle).
	type record struct {
		line  string
		token string // the PII token embedded in this line
	}

	// Generate multiple blocks of PII-bearing content to build a large input.
	records := []record{
		{"2026-06-05T10:15:00Z [INFO ] Request handled user=maria.silva email=maria.silva@example.com", "maria.silva@example.com"},
		{"2026-06-05T10:15:01Z [INFO ] Request handled user=joao.souza email=joao.souza@corp.io", "joao.souza@corp.io"},
		{"2026-06-05T10:15:02Z [INFO ] New account CPF 52998224725 registered via mobile app", "52998224725"},
		{"2026-06-05T10:15:03Z [INFO ] New account CPF 12345678909 created successfully", "12345678909"},
		{"2026-06-05T10:15:04Z [INFO ] New account CPF 98765432100 approved by compliance", "98765432100"},
		{"2026-06-05T10:15:05Z [INFO ] Payment 4532016743529607 authorized for order #8812", "4532016743529607"},
		{"2026-06-05T10:15:06Z [INFO ] Payment 5500000000000004 processed via gateway", "5500000000000004"},
		{"2026-06-05T10:15:07Z [INFO ] Payment 4111111111111111 declined insufficient funds", "4111111111111111"},
		{"2026-06-05T10:15:08Z [INFO ] Client 192.168.1.10 connected from VPN endpoint", "192.168.1.10"},
		{"2026-06-05T10:15:09Z [INFO ] Client 10.0.0.5 established websocket session", "10.0.0.5"},
		{"2026-06-05T10:15:10Z [INFO ] Client 172.16.0.1 disconnected after timeout", "172.16.0.1"},
	}

	for i := 0; i < 6; i++ {
		for _, r := range records {
			buf.WriteString(r.line)
			buf.WriteByte('\n')
		}
		// Noise lines to increase token count and exercise the pool.
		buf.WriteString("2026-06-05T10:15:11Z [DEBUG] cache hit key=session:abc123 ttl=300\n")
		buf.WriteString("2026-06-05T10:15:12Z [DEBUG] db query latency=2ms rows=42\n")
		buf.WriteString("2026-06-05T10:15:13Z [TRACE] rpc call service=payments method=charge\n")
		buf.WriteString("2026-06-05T10:15:14Z [DEBUG] metrics scraped total_requests=15420\n")
		buf.WriteString("2026-06-05T10:15:15Z [INFO ] health check passed, all services healthy\n")
	}
	return buf.Bytes()
}

// printResult displays the anonymization output and detected entities
// for a given analyzer run.
func printResult(label string, output []byte, details analyzer.AnonymizationDetails, elapsed time.Duration) {
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Printf("%s\n", label)
	fmt.Printf("  Elapsed:        %v\n", elapsed)
	fmt.Printf("  Has findings:   %v\n", details.HasFindings)
	fmt.Printf("  Detected:       %v\n", details.DetectedEntities)
	fmt.Printf("  Anonymized:     %v\n", details.AnonymizedEntities)
	fmt.Printf("  Output length:  %d bytes\n", len(output))
	fmt.Println("─────────────────────────────────────────────────────")
}

func main() {
	ctx := context.Background()
	input := largeInput()

	fmt.Printf("Input size: %d bytes\n\n", len(input))

	// ── Rules ─────────────────────────────────────────────────────────
	// Use five default rules plus a custom VIN rule covering common PII types.
	rules := []analyzer.Rule{
		leakspok.DefaultCPFRule,
		leakspok.DefaultCNPJRule,
		leakspok.DefaultEmailRule,
		leakspok.DefaultCreditCardRule,
		leakspok.DefaultIPRule,
		{
			Name:        "vehicle_vin",
			Description: "Vehicle Identification Number",
			Matcher:     pattern.VINMatcher(),
			Settings:    leakspok.DefaultRuleSetting,
		},
	}

	// ── Mode 1: Serial (baseline) ────────────────────────────────────
	{
		logger := slog.Default()
		serialAnalyzer, err := analyzer.MakeByteAnalyzer(ctx, logger, analyzer.RunnerOptions{})
		if err != nil {
			panic(fmt.Sprintf("failed to create serial analyzer: %v", err))
		}
		defer serialAnalyzer.Stop()

		var output bytes.Buffer
		start := time.Now()
		details := serialAnalyzer.Anonymize(ctx, rules, &output, input)
		elapsed := time.Since(start)

		printResult("Serial (baseline)", output.Bytes(), details, elapsed)
		fmt.Println()
	}

	// ── Mode 2: Concurrent Token Processing ──────────────────────────
	// Each token is dispatched to a goroutine from a shared pool.
	// The pool size controls parallelism; results are sorted by token
	// position so output is byte-for-byte identical to serial.
	{
		logger := slog.Default()
		opts := analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                   true,
				ConcurrentTokenProcessing: true,
				TokenPoolSize:             8,
			},
		}
		tokenAnalyzer, err := analyzer.MakeByteAnalyzer(ctx, logger, opts)
		if err != nil {
			panic(fmt.Sprintf("failed to create token-concurrent analyzer: %v", err))
		}
		defer tokenAnalyzer.Stop()

		var output bytes.Buffer
		start := time.Now()
		details := tokenAnalyzer.Anonymize(ctx, rules, &output, input)
		elapsed := time.Since(start)

		printResult("Concurrent Token Processing (pool=8)", output.Bytes(), details, elapsed)
		fmt.Println()
	}

	// ── Mode 3: Concurrent Rule Processing ───────────────────────────
	// Rules are evaluated in parallel via ConcurrentRulesRunner backed
	// by a goroutine pool. Each token runs all rules concurrently;
	// the first match wins (short-circuit semantics preserved per token).
	{
		logger := slog.Default()
		opts := analyzer.RunnerOptions{
			Concurrency: analyzer.ConcurrencyOptions{
				Enabled:                  true,
				ConcurrentRuleProcessing: true,
				RuleRunnerPoolSize:       8,
			},
		}
		ruleAnalyzer, err := analyzer.MakeByteAnalyzer(ctx, logger, opts)
		if err != nil {
			panic(fmt.Sprintf("failed to create rule-concurrent analyzer: %v", err))
		}
		defer ruleAnalyzer.Stop()

		var output bytes.Buffer
		start := time.Now()
		details := ruleAnalyzer.Anonymize(ctx, rules, &output, input)
		elapsed := time.Since(start)

		printResult("Concurrent Rule Processing (pool=8)", output.Bytes(), details, elapsed)
	}

	fmt.Println()
	fmt.Println("All three modes completed successfully.")
	fmt.Println("Note: For small inputs the goroutine overhead may make concurrent")
	fmt.Println("modes slower than serial. The benefit increases with input size")
	fmt.Println("and rule count — try with larger payloads to see the speedup.")
}
