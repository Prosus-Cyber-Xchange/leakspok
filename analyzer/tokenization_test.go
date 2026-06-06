package analyzer_test

import (
	"testing"

	"github.com/ifood/leakspok/analyzer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenIterator(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []analyzer.Token
	}{
		{
			name:     "empty input",
			input:    []byte(""),
			expected: []analyzer.Token{},
		},
		{
			name:  "single token",
			input: []byte("hello"),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
			},
		},
		{
			name:  "two tokens separated by space",
			input: []byte("hello world"),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 6, End: 11},
			},
		},
		{
			name:  "multiple tokens with various spaces",
			input: []byte("one  two   three"),
			expected: []analyzer.Token{
				{Start: 0, End: 3},
				{Start: 5, End: 8},
				{Start: 11, End: 16},
			},
		},
		{
			name:  "tokens separated by commas",
			input: []byte("one,two,three"),
			expected: []analyzer.Token{
				{Start: 0, End: 3},
				{Start: 4, End: 7},
				{Start: 8, End: 13},
			},
		},
		{
			name:  "tokens separated by semicolon",
			input: []byte("one;two;three"),
			expected: []analyzer.Token{
				{Start: 0, End: 3},
				{Start: 4, End: 7},
				{Start: 8, End: 13},
			},
		},
		{
			name:  "tokens with punctuation delimiters",
			input: []byte("hello!world?test(case)"),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 6, End: 11},
				{Start: 12, End: 16},
				{Start: 17, End: 21},
			},
		},
		{
			name:  "tokens with brackets and braces",
			input: []byte("one[two]{three}"),
			expected: []analyzer.Token{
				{Start: 0, End: 3},
				{Start: 4, End: 7},
				{Start: 9, End: 14},
			},
		},
		{
			name:  "tokens with quotes",
			input: []byte(`one"two'three`),
			expected: []analyzer.Token{
				{Start: 0, End: 3},
				{Start: 4, End: 7},
				{Start: 8, End: 13},
			},
		},
		{
			name:  "leading delimiters",
			input: []byte("   hello"),
			expected: []analyzer.Token{
				{Start: 3, End: 8},
			},
		},
		{
			name:  "trailing delimiters",
			input: []byte("hello   "),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
			},
		},
		{
			name:  "tokens with newline delimiter",
			input: []byte("hello\nworld"),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 6, End: 11},
			},
		},
		{
			name:  "tokens with tab delimiter",
			input: []byte("hello\tworld"),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 6, End: 11},
			},
		},
		{
			name:  "tokens with carriage return delimiter",
			input: []byte("hello\rworld"),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 6, End: 11},
			},
		},
		{
			name:  "mixed delimiters",
			input: []byte("hello, world! test?\n case(one) [two]"),
			// remember: a \n inside double-quotes is red as single rune (byte)
			expected: []analyzer.Token{
				{Start: 0, End: 5},   // hello
				{Start: 7, End: 12},  // world
				{Start: 14, End: 18}, // test
				{Start: 21, End: 25}, // case
				{Start: 26, End: 29}, // one
				{Start: 32, End: 35}, // two
			},
		},
		{
			name:  "escaped backslash before regular character is a delimiter",
			input: []byte("hello\\world"),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 6, End: 11},
			},
		},
		{
			name:  "escaped newline is a delimiter",
			input: []byte(`hello\nworld`),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 7, End: 12},
			},
		},
		{
			name:  "escaped tab is a delimiter",
			input: []byte(`hello\tworld`),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 7, End: 12},
			},
		},
		{
			name:  "escaped carriage return is a delimiter",
			input: []byte(`hello\rworld`),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 7, End: 12},
			},
		},
		{
			name:  "multiple escaped sequences in token",
			input: []byte(`hello\n\t\rworld`),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 11, End: 16},
			},
		},
		{
			name:  "escape sequence followed by delimiter",
			input: []byte("hello\\n world"),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 8, End: 13},
			},
		},
		{
			name:  "escape at end of token before space",
			input: []byte(`hello\n world`),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 8, End: 13},
			},
		},
		{
			name:     "only delimiters",
			input:    []byte("   , ; ! ? "),
			expected: []analyzer.Token{},
		},
		{
			name:  "single character token",
			input: []byte("a"),
			expected: []analyzer.Token{
				{Start: 0, End: 1},
			},
		},
		{
			name:  "consecutive delimiter punctuation",
			input: []byte("one,;!?two"),
			expected: []analyzer.Token{
				{Start: 0, End: 3},
				{Start: 7, End: 10},
			},
		},
		{
			name:  "url-like token",
			input: []byte("https://example.com/path"),
			expected: []analyzer.Token{
				{Start: 0, End: 6},
				{Start: 8, End: 19},
				{Start: 20, End: 24},
			},
		},
		{
			name:  "uuid token",
			input: []byte("550e8400-e29b-41d4-a716-446655440000"),
			expected: []analyzer.Token{
				{Start: 0, End: 36},
			},
		},
		{
			name:  "ipv4 address token",
			input: []byte("192.168.1.1"),
			expected: []analyzer.Token{
				{Start: 0, End: 11},
			},
		},
		{
			name:  "number token",
			input: []byte("12345"),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
			},
		},
		{
			name:  "token with hyphens",
			input: []byte("my-token"),
			expected: []analyzer.Token{
				{Start: 0, End: 8},
			},
		},
		{
			name:  "token with underscores",
			input: []byte("my_token"),
			expected: []analyzer.Token{
				{Start: 0, End: 8},
			},
		},
		{
			name:  "token with mixed case",
			input: []byte("MyToken"),
			expected: []analyzer.Token{
				{Start: 0, End: 7},
			},
		},
		{
			name:  "complex real-world log line",
			input: []byte("ERROR: Failed to authenticate test@email.com (id: 12345)"),
			expected: []analyzer.Token{
				{Start: 0, End: 6},   // ERROR:
				{Start: 7, End: 13},  // Failed
				{Start: 14, End: 16}, // to
				{Start: 17, End: 29}, // authenticate
				{Start: 30, End: 44}, // test@email.com
				{Start: 46, End: 49}, // id:
				{Start: 50, End: 55}, // 12345
			},
		},
		{
			name:  "token followed by multiple delimiter types",
			input: []byte("word,; !?"),
			expected: []analyzer.Token{
				{Start: 0, End: 4},
			},
		},
		{
			name:     "empty result with only escaped delimiters",
			input:    []byte("\\n\\t\\r"),
			expected: []analyzer.Token{},
		},
		{
			name:  "token with escaped delimiter not followed by control char",
			input: []byte(`hello\xworld`),
			expected: []analyzer.Token{
				{Start: 0, End: 5},
				{Start: 6, End: 12},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tokens []analyzer.Token

			// Use range-over-func to collect tokens
			for token := range analyzer.TokenIterator(tt.input) {
				tokens = append(tokens, token)
			}

			require.Equal(t, len(tt.expected), len(tokens), "expected %d tokens, got %d", len(tt.expected), len(tokens))

			for i, expected := range tt.expected {
				actual := tokens[i]
				assert.Equal(t, expected.Start, actual.Start, "token %d: start position mismatch", i)
				assert.Equal(t, expected.End, actual.End, "token %d: end position mismatch", i)

				// Verify the token content matches the input bytes
				actualContent := tt.input[actual.Start:actual.End]
				assert.NotEmpty(t, actualContent, "token %d: content should not be empty", i)
			}
		})
	}
}

func TestTokenIterator_EarlyExit(t *testing.T) {
	// Test that the iterator stops when yield returns false
	input := []byte("one two three four five")

	count := 0
	for token := range analyzer.TokenIterator(input) {
		count++
		if count == 2 {
			// Stop iteration after 2 tokens
			break
		}
		_ = token
	}

	assert.Equal(t, 2, count, "iterator should stop when break is called")
}

func TestTokenIterator_NilInput(t *testing.T) {
	// TokenIterator should produce zero tokens on nil input
	var tokens []analyzer.Token
	for token := range analyzer.TokenIterator(nil) {
		tokens = append(tokens, token)
	}
	assert.Empty(t, tokens, "TokenIterator should produce zero tokens on nil input")
}

func TestTokenIterator_VeryLargeInput(t *testing.T) {
	// Create a very large input (100KB) with periodic delimiters
	largeInput := make([]byte, 100000)
	for i := range largeInput {
		if i%20 == 0 {
			largeInput[i] = ' ' // delimiter every 20 bytes
		} else {
			largeInput[i] = 'a' + byte(i%26)
		}
	}

	var tokens []analyzer.Token
	for token := range analyzer.TokenIterator(largeInput) {
		tokens = append(tokens, token)
	}

	// Should produce many tokens, roughly 100000/20 = 5000
	assert.Greater(t, len(tokens), 4000, "very large input should produce many tokens")

	// Verify content of first token (position 0 is space, so first token starts at 1)
	assert.Equal(t, 1, tokens[0].Start)
	assert.Equal(t, 20, tokens[0].End)
	assert.Equal(t, string(largeInput[1:20]), string(largeInput[tokens[0].Start:tokens[0].End]))
}

func TestTokenIterator_UnicodeDelimiters(t *testing.T) {
	// Input with unicode characters acting as token content (not delimiters)
	// but with standard delimiters mixed in
	input := []byte("café, naïve résumé")

	var tokenContents []string
	for token := range analyzer.TokenIterator(input) {
		tokenContents = append(tokenContents, string(input[token.Start:token.End]))
	}

	// Tokenizer should split on standard delimiters (comma, space)
	// but keep unicode characters within tokens
	assert.Equal(t, []string{"café", "naïve", "résumé"}, tokenContents)
}

func TestTokenIterator_OnlyDelimitersEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"only spaces", []byte("     ")},
		{"only punctuation", []byte(",;!?()[]{}\"'/\\")},
		{"mix of spaces and punctuation", []byte(" , ; ! ? ( ) ")},
		{"newlines and tabs only", []byte("\n\t\r\n\t")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tokens []analyzer.Token
			for token := range analyzer.TokenIterator(tt.input) {
				tokens = append(tokens, token)
			}
			assert.Empty(t, tokens, "input with only delimiters should produce zero tokens")
		})
	}
}

func BenchmarkTokenIterator(b *testing.B) {
	// Medium-sized realistic input
	input := []byte("Contact user@example.com or call +1-555-123-4567. " +
		"Credit card: 4111111111111111. CPF: 529.982.247-25. CNPJ: 11.222.333/0001-81. " +
		"IP: 192.168.1.100. Visit https://www.example.com/page. " +
		"SSN: 123-45-6789. UUID: 550e8400-e29b-41d4-a716-446655440000.")

	b.ResetTimer()

	for b.Loop() {
		tokenCount := 0
		for range analyzer.TokenIterator(input) {
			tokenCount++
		}
		_ = tokenCount
	}
}

func BenchmarkTokenIteratorLarge(b *testing.B) {
	// ~5KB of realistic text
	base := "Contact user@example.com or call +1-555-123-4567. Credit card: 4111111111111111. CPF: 529.982.247-25. "
	largeInput := []byte("")
	for range 50 {
		largeInput = append(largeInput, []byte(base)...)
	}

	b.ResetTimer()

	for b.Loop() {
		tokenCount := 0
		for range analyzer.TokenIterator(largeInput) {
			tokenCount++
		}
		_ = tokenCount
	}
}

func TestTokenIterator_CorrectByteExtraction(t *testing.T) {
	// Verify that token positions correctly reference the input bytes
	tests := []struct {
		name     string
		input    []byte
		expected []string // expected token contents
	}{
		{
			name:     "simple tokens",
			input:    []byte("hello world"),
			expected: []string{"hello", "world"},
		},
		{
			name:     "tokens with special characters",
			input:    []byte("[PROMOTION] phone: 555-1234"),
			expected: []string{"PROMOTION", "phone:", "555-1234"},
		},
		{
			name:     "tokens with numbers and symbols",
			input:    []byte("IPv4: 192.168.1.1; port: 8080"),
			expected: []string{"IPv4:", "192.168.1.1", "port:", "8080"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var contents []string

			for token := range analyzer.TokenIterator(tt.input) {
				content := string(tt.input[token.Start:token.End])
				contents = append(contents, content)
			}

			assert.Equal(t, tt.expected, contents, "token contents should match expected values")
		})
	}
}
