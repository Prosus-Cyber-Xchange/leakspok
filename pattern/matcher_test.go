package pattern_test

import (
	"context"
	"testing"

	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhone(t *testing.T) {
	matcher := pattern.PhoneMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid US phone", "+1-555-123-4567", true},
		{"Valid phone with parentheses", "(555) 123-4567", true},
		{"Valid international phone", "+44 20 7946 0958", true},
		{"Valid phone no formatting", "5551234567", true},
		{"Valid phone with dash", "99004-6519", true},
		{"Email should be excluded", "test@example.com", false},
		{"Filename should be excluded", "file-123-456.txt", false},
		{"Repeating numbers should be excluded", "1111111111", false},
		{"Invalid short number", "123", false},
		{"Invalid format", "abc-def-ghij", false},
	}

	assert.Equal(t, pattern.EntityPhone, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestLink(t *testing.T) {
	matcher := pattern.LinkMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid HTTP URL", "http://example.com", true},
		{"Valid HTTPS URL", "https://www.google.com", true},
		{"Valid URL without protocol", "www.example.com", true},
		{"Valid URL with path", "https://example.com/path/to/resource", true},
		{"Email should be excluded", "user@example.com", false},
		{"Plain text", "hello world", false},
		{"Invalid URL", "not-a-url", false},
	}

	assert.Equal(t, pattern.EntityLink, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestSSN(t *testing.T) {
	matcher := pattern.SSNMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid SSN with dashes", "123-45-6789", true},
		{"Valid SSN with spaces", "123 45 6789", true},
		{"Phone number should be excluded", "555-123-4567", false},
		{"Filename should be excluded", "file-123-456.txt", false},
		{"Repeating numbers should be excluded", "11111-11-1111", false},
		{"Invalid format", "12-345-6789", false},
		{"Invalid format no separators", "123456789", false},
		{"Invalid characters", "abc-de-fghi", false},
	}

	assert.Equal(t, pattern.EntitySSN, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestEmail(t *testing.T) {
	matcher := pattern.EmailMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid email", "user@example.com", true},
		{"Valid email with subdomain", "test@mail.example.com", true},
		{"Valid email with plus", "user+tag@example.com", true},
		{"Valid email with dots", "first.last@example.com", true},
		{"Invalid missing @", "userexample.com", false},
		{"Invalid missing domain", "user@", false},
		{"Invalid missing user", "@example.com", false},
		{"Plain text", "hello world", false},
	}

	assert.Equal(t, pattern.EntityEmail, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestIP(t *testing.T) {
	matcher := pattern.IPMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid IPv4", "192.168.1.1", true},
		{"Valid IPv4 localhost", "127.0.0.1", true},
		{"Valid IPv6", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"Valid IPv6 compressed", "::1", true},
		{"Valid IPv6 short", "2001:db8::1", true},
		{"Invalid IPv4", "256.256.256.256", false},
		{"Invalid format", "192.168.1", false},
		{"Plain text", "hello world", false},
	}

	assert.Equal(t, pattern.EntityIPAddress, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestIPv4(t *testing.T) {
	matcher := pattern.IPv4Matcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid IPv4", "192.168.1.1", true},
		{"Valid IPv4 localhost", "127.0.0.1", true},
		{"Valid IPv4 edge case", "0.0.0.0", true},
		{"Invalid IPv4", "256.256.256.256", false},
		{"IPv6 should not match", "2001:db8::1", false},
		{"Invalid format", "192.168.1", false},
	}

	assert.Equal(t, pattern.EntityIPAddress, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestIPv6(t *testing.T) {
	matcher := pattern.IPv6Matcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid IPv6 full", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"Valid IPv6 compressed", "::1", true},
		{"Valid IPv6 short", "2001:db8::1", true},
		{"IPv4 should not match", "192.168.1.1", false},
		{"Invalid format", ":1", false},
	}

	assert.Equal(t, pattern.EntityIPAddress, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestCreditCard(t *testing.T) {
	matcher := pattern.CreditCardMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid Visa card", "4111111111111111", true},
		{"Valid Mastercard", "5200100000002805", true},
		{"Valid card with spaces", "4111 1111 1111 1111", true},
		{"Valid card with dashes", "4111-1111-1111-1111", true},
		{"UUID should be excluded", "123e4567-e89b-12d3-a456-426614174000", false},
		{"Repeating numbers should be excluded", "1111111111111111", false},
		{"Test card should be excluded", "6011111111111117", false},
		{"Invalid short number", "411111111", false},
		{"Invalid format", "abcd-efgh-ijkl-mnop", false},
	}

	assert.Equal(t, pattern.EntityCreditCard, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestAddress(t *testing.T) {
	matcher := pattern.AddressMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid street address with zip", "123 Main Street 12345", true},
		{"Valid avenue with zip", "456 Oak Avenue 90210", true},
		{"Valid PO Box with zip", "P.O. Box 123 12345", true},
		{"Street without zip", "123 Main Street", false},
		{"Zip without street", "12345", false},
		{"Invalid format", "hello world", false},
	}

	assert.Equal(t, pattern.EntityAddress, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestBankInfo(t *testing.T) {
	matcher := pattern.BankInfoMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid IBAN", "GB82 WEST 1234 5698 7654 32", true},
		{"Valid IBAN no spaces", "GB82WEST12345698765432", true},
		{"Valid German IBAN", "DE89 3704 0044 0532 0130 00", true},
		{"Repeating numbers should be excluded", "GB82111111111111111111", false},
		{"Invalid format", "INVALID123456", false},
		{"Plain text", "hello world", false},
	}

	assert.Equal(t, pattern.EntityBankInfo, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestUUID(t *testing.T) {
	matcher := pattern.UUIDMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid UUID v4", "123e4567-e89b-12d3-a456-426614174000", true},
		{"Valid GUID", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", true},
		{"Valid UUID without dashes", "123e4567e89b12d3a456426614174000", true},
		{"Filename should be excluded", "file-123e4567-e89b.txt", false},
		{"Invalid format", "invalid-uuid-format", false},
		{"Too short", "123e4567", false},
	}

	assert.Equal(t, pattern.EntityUUID, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestCPF(t *testing.T) {
	matcher := pattern.CPFMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid CPF with dots and dash", "123.456.789-09", true},
		{"Valid CPF numbers only", "12345678909", true},
		{"Valid CPF partial format", "123.456.78909", true},
		{"Valid CPF with dash only", "123456789-09", true},
		{"Invalid format", "123.456.789", false},
		{"Invalid characters", "abc.def.ghi-jk", false},
		{"Too short", "123456789", false},
	}

	assert.Equal(t, pattern.EntityCPF, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestCNPJ(t *testing.T) {
	matcher := pattern.CNPJMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid CNPJ with formatting", "12.345.678/0001-95", true},
		{"Valid CNPJ numbers only", "12345678000195", true},
		{"Invalid format", "12.345.678/0001", false},
		{"Invalid characters", "ab.cde.fgh/ijkl-mn", false},
		{"Too short", "123456780001", false},
	}

	assert.Equal(t, pattern.EntityCNPJ, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestBrazilianPII(t *testing.T) {
	matcher := pattern.BrazilianPIIMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid CPF", "123.456.789-09", true},
		{"Valid CNPJ", "12.345.678/0001-95", true},
		{"Valid CPF numbers only", "12345678909", true},
		{"Valid CNPJ numbers only", "12345678000195", true},
		{"Invalid format", "123.456.789", false},
		{"Plain text", "hello world", false},
	}

	assert.Equal(t, "BRAZILIAN_PII", string(matcher.Entity()))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestNewPatternMatcher(t *testing.T) {
	t.Run("Entity returns assigned entity", func(t *testing.T) {
		customEntity := pattern.Entity("CUSTOM_ENTITY")
		matcher := pattern.NewPatternMatcher(customEntity, pattern.PatternFunc(func(_ context.Context, input []byte) bool {
			return len(input) > 0
		}))
		assert.Equal(t, customEntity, matcher.Entity())
	})

	t.Run("Entity with standard entity type", func(t *testing.T) {
		matcher := pattern.NewPatternMatcher(pattern.EntityEmail, pattern.PatternFunc(func(_ context.Context, input []byte) bool {
			return true
		}))
		assert.Equal(t, pattern.EntityEmail, matcher.Entity())
	})

	t.Run("Match delegates to pattern and returns true", func(t *testing.T) {
		matcher := pattern.NewPatternMatcher("TEST", pattern.PatternFunc(func(_ context.Context, input []byte) bool {
			return string(input) == "match"
		}))
		assert.True(t, matcher.Match(context.Background(), []byte("match")))
	})

	t.Run("Match delegates to pattern and returns false", func(t *testing.T) {
		matcher := pattern.NewPatternMatcher("TEST", pattern.PatternFunc(func(_ context.Context, input []byte) bool {
			return string(input) == "match"
		}))
		assert.False(t, matcher.Match(context.Background(), []byte("no match")))
	})

	t.Run("Match with empty input", func(t *testing.T) {
		matcher := pattern.NewPatternMatcher("TEST", pattern.PatternFunc(func(_ context.Context, input []byte) bool {
			return len(input) == 0
		}))
		assert.True(t, matcher.Match(context.Background(), []byte("")))
	})

	t.Run("Match with nil input", func(t *testing.T) {
		matcher := pattern.NewPatternMatcher("TEST", pattern.PatternFunc(func(_ context.Context, input []byte) bool {
			return input == nil
		}))
		assert.True(t, matcher.Match(context.Background(), nil))
	})

	t.Run("Entity with empty string", func(t *testing.T) {
		matcher := pattern.NewPatternMatcher("", pattern.PatternFunc(func(_ context.Context, input []byte) bool {
			return true
		}))
		assert.Equal(t, pattern.Entity(""), matcher.Entity())
	})
}

func TestVIN(t *testing.T) {
	matcher := pattern.VINMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid VIN uppercase", "1HGBH41JXMN109186", true},
		{"Valid VIN lowercase", "1hgbh41jxmn109186", true},
		{"Valid VIN mixed case", "1hgbH41JXmn109186", true},
		{"Valid VIN second example", "WDBJF65F9VA345942", true},
		{"Valid VIN third example", "JS1GN7AA312104247", true},
		{"Valid VIN with digits only no I/O/Q", "12345678901234567", true},
		{"Invalid contains I", "1HGBH41JXMN10I186", false},
		{"Invalid contains O", "1HGBH41JXMN10O186", false},
		{"Invalid contains Q", "1HGBH41JXMN10Q186", false},
		{"Invalid contains lowercase i", "ihgbh41jxmn109186", false},
		{"Invalid contains lowercase o", "ohgbh41jxmn109186", false},
		{"Invalid contains lowercase q", "qhgbh41jxmn109186", false},
		{"Invalid too short 16 chars", "1HGBH41JXMN10918", false},
		{"Invalid too long 18 chars", "1HGBH41JXMN1091862", false},
		{"Invalid empty", "", false},
		{"Invalid nil", "", false},
		{"Invalid special characters", "1HGBH41JX-N109186", false},
		{"Invalid spaces", "1HGB H41JXMN109186", false},
		{"Invalid short 1 char", "A", false},
		{"Plain text", "hello world", false},
		{"Random long string not VIN", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", false},
	}

	assert.Equal(t, pattern.EntityVIN, matcher.Entity())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestAllMatchersEdgeCases(t *testing.T) {
	// Build a slice of all public matchers for edge case testing.
	matchers := []struct {
		name    string
		matcher pattern.PatternMatcher
	}{
		{"PhoneMatcher", pattern.PhoneMatcher()},
		{"LinkMatcher", pattern.LinkMatcher()},
		{"SSNMatcher", pattern.SSNMatcher()},
		{"EmailMatcher", pattern.EmailMatcher()},
		{"IPMatcher", pattern.IPMatcher()},
		{"IPv4Matcher", pattern.IPv4Matcher()},
		{"IPv6Matcher", pattern.IPv6Matcher()},
		{"CreditCardMatcher", pattern.CreditCardMatcher()},
		{"AddressMatcher", pattern.AddressMatcher()},
		{"BankInfoMatcher", pattern.BankInfoMatcher()},
		{"UUIDMatcher", pattern.UUIDMatcher()},
		{"CPFMatcher", pattern.CPFMatcher()},
		{"CNPJMatcher", pattern.CNPJMatcher()},
		{"BrazilianPIIMatcher", pattern.BrazilianPIIMatcher()},
		{"VINMatcher", pattern.VINMatcher()},
	}

	ctx := context.Background()

	// Generate 10KB+ payload of random text (no PII content)
	// Use 'z' to avoid matching hex-based matchers (UUIDV2 matches 32+ consecutive hex chars)
	veryLong := make([]byte, 10240)
	for i := range veryLong {
		veryLong[i] = 'z'
	}

	for _, m := range matchers {
		t.Run(m.name+"/empty_slice", func(t *testing.T) {
			result := m.matcher.Match(ctx, []byte{})
			assert.False(t, result, "%s should not match empty byte slice", m.name)
		})

		t.Run(m.name+"/nil_slice", func(t *testing.T) {
			result := m.matcher.Match(ctx, nil)
			assert.False(t, result, "%s should not match nil byte slice", m.name)
		})

		t.Run(m.name+"/unicode", func(t *testing.T) {
			unicodeInput := []byte("日本語のテストÜber caffè naïve résumé")
			result := m.matcher.Match(ctx, unicodeInput)
			assert.False(t, result, "%s should not match plain unicode text", m.name)
		})

		t.Run(m.name+"/very_long", func(t *testing.T) {
			result := m.matcher.Match(ctx, veryLong)
			assert.False(t, result, "%s should not match 10KB random text", m.name)
		})

		t.Run(m.name+"/single_byte", func(t *testing.T) {
			result := m.matcher.Match(ctx, []byte("a"))
			assert.False(t, result, "%s should not match single byte token", m.name)
		})
	}
}

func TestAllMatchersUnicodeInputs(t *testing.T) {
	// Test each matcher with realistic unicode inputs containing actual PII
	tests := []struct {
		name    string
		matcher pattern.PatternMatcher
		input   []byte
		expect  bool
	}{
		{
			name:    "Email with unicode domain",
			matcher: pattern.EmailMatcher(),
			input:   []byte("user@münchen.de"),
			expect:  true,
		},
		{
			name:    "CPF with only-numeric input",
			matcher: pattern.CPFMatcher(),
			input:   []byte("529.982.247-25"),
			expect:  true,
		},
		{
			name:    "Phone in unicode context",
			matcher: pattern.PhoneMatcher(),
			input:   []byte("Teléfono: +1-555-123-4567"),
			expect:  true,
		},
		{
			name:    "IPv4 in unicode context",
			matcher: pattern.IPMatcher(),
			input:   []byte("アドレス 192.168.1.1 です"),
			expect:  false,
		},
		{
			name:    "Credit card unicode text",
			matcher: pattern.CreditCardMatcher(),
			input:   []byte("卡号 4111111111111111"),
			expect:  false,
		},
		{
			name:    "VIN with unicode prefix",
			matcher: pattern.VINMatcher(),
			input:   []byte("VIN: 1HGBH41JXMN109186"),
			expect:  false,
		},
		{
			name:    "UUID unicode",
			matcher: pattern.UUIDMatcher(),
			input:   []byte("550e8400-e29b-41d4-a716-446655440000"),
			expect:  true,
		},
		{
			name:    "SSN unicode text",
			matcher: pattern.SSNMatcher(),
			input:   []byte("социальный 123-45-6789 номер"),
			expect:  true,
		},
		{
			name:    "IBAN unicode",
			matcher: pattern.BankInfoMatcher(),
			input:   []byte("IBAN: GB82WEST12345698765432"),
			expect:  true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.matcher.Match(ctx, tt.input)
			assert.Equal(t, tt.expect, result, "Input: %s", tt.input)
		})
	}
}

func BenchmarkEmailMatcher(b *testing.B) {
	matcher := pattern.EmailMatcher()
	ctx := context.Background()

	// Mix of valid and invalid emails for realistic workload
	inputs := [][]byte{
		[]byte("user@example.com"),
		[]byte("john.doe@company.org"),
		[]byte("not-an-email"),
		[]byte("support@sub.domain.co.uk"),
		[]byte("plain text"),
	}

	b.ResetTimer()

	for b.Loop() {
		for _, input := range inputs {
			_ = matcher.Match(ctx, input)
		}
	}
}

func BenchmarkCPFMatcher(b *testing.B) {
	matcher := pattern.CPFMatcher()
	ctx := context.Background()

	inputs := [][]byte{
		[]byte("529.982.247-25"),
		[]byte("111.444.777-35"),
		[]byte("12345678901"),
		[]byte("not a cpf"),
		[]byte("000.000.000-00"),
	}

	b.ResetTimer()

	for b.Loop() {
		for _, input := range inputs {
			_ = matcher.Match(ctx, input)
		}
	}
}

func BenchmarkCNPJMatcher(b *testing.B) {
	matcher := pattern.CNPJMatcher()
	ctx := context.Background()

	inputs := [][]byte{
		[]byte("11.222.333/0001-81"),
		[]byte("11222333000181"),
		[]byte("00.000.000/0000-00"),
		[]byte("not a cnpj"),
		[]byte("12345678901234"),
	}

	b.ResetTimer()

	for b.Loop() {
		for _, input := range inputs {
			_ = matcher.Match(ctx, input)
		}
	}
}

func BenchmarkCreditCardMatcher(b *testing.B) {
	matcher := pattern.CreditCardMatcher()
	ctx := context.Background()

	// Mix of valid and invalid credit card numbers (Visa, Mastercard, etc.)
	inputs := [][]byte{
		[]byte("4111111111111111"),
		[]byte("5500000000000004"),
		[]byte("340000000000009"),
		[]byte("6011000000000004"),
		[]byte("1234567890123456"),
		[]byte("not a card"),
	}

	b.ResetTimer()

	for b.Loop() {
		for _, input := range inputs {
			_ = matcher.Match(ctx, input)
		}
	}
}

func BenchmarkIPMatcher(b *testing.B) {
	matcher := pattern.IPMatcher()
	ctx := context.Background()

	inputs := [][]byte{
		[]byte("192.168.1.1"),
		[]byte("10.0.0.255"),
		[]byte("2001:db8::1"),
		[]byte("::1"),
		[]byte("not.an.ip.address"),
		[]byte("999.999.999.999"),
	}

	b.ResetTimer()

	for b.Loop() {
		for _, input := range inputs {
			_ = matcher.Match(ctx, input)
		}
	}
}

func TestHaltLangDetect(t *testing.T) {
	haltPattern := pattern.HaltLangDetect()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"UUID should halt", "123e4567-e89b-12d3-a456-426614174000", true},
		{"URL should halt", "https://example.com", true},
		{"Email should halt", "user@example.com", true},
		{"Credit card should halt", "4111111111111111", true},
		{"Plain text should not halt", "hello world", false},
		{"Random numbers should not halt", "123456", false},
	}

	require.NotNil(t, haltPattern)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := haltPattern.Match(context.Background(), []byte(tt.input))
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}
