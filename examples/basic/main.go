// Basic example: demonstrates simple string anonymization with leakspok.
//
// This program shows the simplest path to detect and anonymize PII in a string:
//  1. Select default detection rules (CPF, email, credit card)
//  2. Create a StringAnalyzer backed by a serial rule runner
//  3. Call Anonymize to produce a cleaned string
//
// Run with:
//
//	go run ./examples/basic/
//
// Output shows which entity types were detected and the anonymized result.
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Prosus-Cyber-Xchange/leakspok"
	"github.com/Prosus-Cyber-Xchange/leakspok/analyzer"
	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
)

func main() {
	// 1. Choose the rules you want to enforce.
	//    Default rules are pre-configured with the REDACT strategy.
	rules := []analyzer.Rule{
		leakspok.DefaultCPFRule,        // Brazilian CPF (e.g. 529.982.247-25)
		leakspok.DefaultEmailRule,      // Email addresses
		leakspok.DefaultCreditCardRule, // Credit card numbers
	}

	// 2. Build the analyzer.
	//    The SerialRulesRunner evaluates rules one at a time — suitable for
	//    most workloads. Pass a discard logger or slog.Default().
	logger := slog.Default()
	runner := analyzer.NewSerialRulesRuner(logger, analyzercache.NewNoopRuleMatchingCache())
	sa := analyzer.NewStringAnalyzer(logger, runner)

	// 3. Define the input to scan.
	//    It contains multiple types of PII mixed with regular text.
	input := "" +
		"User report:\n" +
		"  Name: Maria Silva\n" +
		"  CPF: 529.982.247-25\n" +
		"  Email: maria.silva@example.com\n" +
		"  Card: 4532-0167-4352-9607\n"

	// 4. Anonymize in one call.
	//    The returned string has all detected PII replaced with <REDACTED>.
	result, details := sa.Anonymize(context.Background(), rules, input)

	// 5. Inspect the results.
	fmt.Println("=== Anonymization Results ===")
	fmt.Printf("Has findings: %v\n", details.HasFindings)
	if details.HasFindings {
		fmt.Printf("Detected entities:   %v\n", details.DetectedEntities)
		fmt.Printf("Anonymized entities: %v\n", details.AnonymizedEntities)
	}
	fmt.Println("--- Anonymized output ---")
	fmt.Print(result)
}
