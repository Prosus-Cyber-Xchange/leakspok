// Custom rules example: demonstrates custom rule creation, exception handling,
// strategy selection, and pattern composition with leakspok.
//
// This program shows how to build application-specific detection rules:
//  1. A custom "API Key" rule that detects tokens starting with "sk-" (REDACT)
//  2. An exception that whitelists test keys (sk-test-*)
//  3. A custom "Order ID" rule that uses MASK instead of REDACT
//  4. Pattern composition with And/Or/Not to build complex conditions
//
// Run with:
//
//	go run ./examples/custom-rules/
//
// Output shows how different rules and strategies produce different
// anonymization behaviors on the same input.
package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/Prosus-Cyber-Xchange/leakspok/analyzer"
	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
)

func main() {
	// ── Custom entity: API Key ────────────────────────────────────────
	// NewEntity creates an Entity value for a custom PII type.
	apiKeyEntity := pattern.Entity("API_KEY")

	// Custom pattern: matches tokens that start with "sk-" followed by at
	// least 20 alphanumeric characters. Uses pattern composition to combine
	// a prefix check with a minimum-length requirement.
	minLength := pattern.NewBasePattern(
		"len>=24",
		func(_ context.Context, b []byte) bool { return len(b) >= 24 },
	)
	hasPrefix := pattern.NewBasePattern(
		"startsWith(sk-)",
		func(_ context.Context, b []byte) bool {
			return bytes.HasPrefix(b, []byte("sk-"))
		},
	)
	apiKeyPattern := pattern.And(hasPrefix, minLength)
	apiKeyMatcher := pattern.NewPatternMatcher(apiKeyEntity, apiKeyPattern)

	// ── Exception: whitelist test keys ────────────────────────────────
	// Any API key starting with "sk-test-" is a known-safe test key.
	// The exception matcher uses a StartsWith pattern; when a token matches
	// this exception, the rule is NOT applied to that token.
	testKeyException := analyzer.Exception{
		Reason: "Test API keys (sk-test-*) are safe and can appear in logs",
		Matcher: pattern.NewPatternMatcher(
			pattern.Entity("TEST_API_KEY"),
			pattern.NewBasePattern(
				"startsWith(sk-test-)",
				func(_ context.Context, b []byte) bool {
					return bytes.HasPrefix(b, []byte("sk-test-"))
				},
			),
		),
	}

	// ── Rule 1: API Key with REDACT strategy ──────────────────────────
	// This rule detects production API keys and replaces them entirely
	// with a placeholder. Test keys are excepted.
	apiKeyRule := analyzer.Rule{
		Name:        "api_key",
		Description: "Production API keys (sk-*, excluding sk-test-*)",
		Matcher:     apiKeyMatcher,
		Exceptions:  []analyzer.Exception{testKeyException},
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact: &analyzer.RedactSettings{
				Placeholder: "[REDACTED_API_KEY]",
			},
		},
	}

	// ── Rule 2: Order ID with MASK strategy ───────────────────────────
	// This rule detects tokens matching "ORD-" followed by 8+ digits and
	// masks them — keeping the prefix visible but hiding the numeric part.
	// Uses Or composition: matches either "ORD-" or "PO-" prefix.
	ordPrefix := pattern.NewBasePattern(
		"startsWith(ORD-)",
		func(_ context.Context, b []byte) bool {
			return bytes.HasPrefix(b, []byte("ORD-"))
		},
	)
	poPrefix := pattern.NewBasePattern(
		"startsWith(PO-)",
		func(_ context.Context, b []byte) bool {
			return bytes.HasPrefix(b, []byte("PO-"))
		},
	)
	minDigits := pattern.NewBasePattern(
		"len>=12",
		func(_ context.Context, b []byte) bool { return len(b) >= 12 },
	)
	orderIDPattern := pattern.And(pattern.Or(ordPrefix, poPrefix), minDigits)
	orderIDMatcher := pattern.NewPatternMatcher(
		pattern.Entity("ORDER_ID"),
		orderIDPattern,
	)

	orderIDRule := analyzer.Rule{
		Name:        "order_id",
		Description: "Order/Purchase Order IDs — mask the numeric suffix",
		Matcher:     orderIDMatcher,
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.MASK,
			Mask: &analyzer.MaskSettings{
				MaskingChar: "*",
				MaxSize:     4, // Keep first 4 chars visible (e.g., ORD-)
			},
		},
	}

	// ── Rule 3: Internal secret with NOT composition ──────────────────
	// Detects tokens matching a hex pattern (like an API secret) but NOT
	// matching the UUID format, so UUIDs are left alone. Uses And + Not.
	hexCharPattern := pattern.NewBasePattern(
		"hexCharsOnly",
		func(_ context.Context, b []byte) bool {
			for _, c := range b {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
					return false
				}
			}
			return true
		},
	)
	longToken := pattern.NewBasePattern(
		"len>=32",
		func(_ context.Context, b []byte) bool { return len(b) >= 32 },
	)

	secretPattern := pattern.And(
		pattern.And(hexCharPattern, longToken),
		pattern.Not(pattern.NewBasePattern(
			"isUUID",
			pattern.MatchUUID,
		)),
	)
	secretMatcher := pattern.NewPatternMatcher(
		pattern.Entity("INTERNAL_SECRET"),
		secretPattern,
	)

	secretRule := analyzer.Rule{
		Name:        "internal_secret",
		Description: "Long hex tokens that are not UUIDs — potential internal secrets",
		Matcher:     secretMatcher,
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact: &analyzer.RedactSettings{
				Placeholder: "[SECRET]",
			},
		},
	}

	// ── Build the analyzer and run ────────────────────────────────────
	logger := slog.Default()
	runner := analyzer.NewSerialRulesRuner(logger, analyzercache.NewNoopRuleMatchingCache())
	sa := analyzer.NewStringAnalyzer(logger, runner)

	// Input demonstrates all rule behaviors:
	//   - sk-live-abcd1234efgh5678ijkl  → REDACT (api_key with REDACT strategy)
	//   - sk-test-0000deadbeefcafe0000 → kept (exception for test keys)
	//   - ORD-20250605123              → MASK (order_id with MASK strategy)
	//   - PO-987654321098              → MASK (order_id with MASK strategy)
	//   - af37c8b92d1e4f6a8b3c1d9e2f4a7c6b0 → REDACT (internal_secret, not UUID)
	//   - 550e8400-e29b-41d4-a716-446655440000 → kept (UUID, excluded by Not)
	rules := []analyzer.Rule{apiKeyRule, orderIDRule, secretRule}

	input := "" +
		"Deployment report:\n" +
		"  API Key: sk-live-abcd1234efgh5678ijkl\n" +
		"  Test Key: sk-test-0000deadbeefcafe0000\n" +
		"  Order: ORD-20250605123\n" +
		"  Purchase: PO-987654321098\n" +
		"  Secret: af37c8b92d1e4f6a8b3c1d9e2f4a7c6b0\n" +
		"  Request ID: 550e8400-e29b-41d4-a716-446655440000\n"

	result, details := sa.Anonymize(context.Background(), rules, input)

	// ── Print results ─────────────────────────────────────────────────
	fmt.Println("╔════════════════════════════════════════════════════╗")
	fmt.Println("║     Custom Rules — Anonymization Results          ║")
	fmt.Println("╚════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Has findings: %v\n", details.HasFindings)
	fmt.Printf("Detected entities:   %v\n", details.DetectedEntities)
	fmt.Printf("Anonymized entities: %v\n", details.AnonymizedEntities)
	fmt.Println()

	fmt.Println("─── Rules Applied ───")
	for _, r := range rules {
		strategy := "REDACT"
		if r.Settings.Strategy == analyzer.MASK {
			strategy = "MASK"
		}
		exceptionNote := ""
		if len(r.Exceptions) > 0 {
			exceptionNote = fmt.Sprintf(" (with %d exception(s))", len(r.Exceptions))
		}
		fmt.Printf("  • %s [%s]%s\n", r.Name, strategy, exceptionNote)
	}
	fmt.Println()

	fmt.Println("─── Anonymized Output ───")
	fmt.Println("Observe:")
	fmt.Println("  - sk-live-... is REDACTED entirely")
	fmt.Println("  - sk-test-... is preserved (exception)")
	fmt.Println("  - ORD-... and PO-... are MASKED (first 4 chars visible)")
	fmt.Println("  - af37c8b... (hex secret) is REDACTED")
	fmt.Println("  - The UUID is left intact (excluded by Not composition)")
	fmt.Println()
	fmt.Print(result)
}
