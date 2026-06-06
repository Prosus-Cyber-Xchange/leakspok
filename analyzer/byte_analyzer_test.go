package analyzer_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/ifood/leakspok/analyzer"
	analyzercache "github.com/ifood/leakspok/analyzer/cache"
	analyzermock "github.com/ifood/leakspok/analyzer/mocks"
	"github.com/ifood/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Helper function to create email matcher
func newEmailMatcher(ctrl *gomock.Controller) *analyzermock.MockMatcher {
	mock := analyzermock.NewMockMatcher(ctrl)
	mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
		return bytes.Contains(input, []byte("@")) && bytes.Contains(input, []byte("."))
	}).AnyTimes()
	return mock
}

// Helper function to create phone matcher
func newPhoneMatcher(ctrl *gomock.Controller) *analyzermock.MockMatcher {
	mock := analyzermock.NewMockMatcher(ctrl)
	mock.EXPECT().Entity().Return(pattern.EntityPhone).AnyTimes()
	mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
		// Simple check for sequences that look like phone numbers
		return len(input) >= 10 && bytes.ContainsAny(input, "0123456789")
	}).AnyTimes()
	return mock
}

// Helper function to create exception matcher
func newExceptionMatcher(ctrl *gomock.Controller, matchValue string) *analyzermock.MockMatcher {
	mock := analyzermock.NewMockMatcher(ctrl)
	mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
		return string(input) == matchValue
	}).AnyTimes()
	return mock
}

func TestNewByteAnalyzer(t *testing.T) {
	ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	// Analyzer should be properly initialized
	assert.NotNil(t, ba)
}

func TestByteAnalyzer_Anonymize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name            string
		rules           []analyzer.Rule
		input           []byte
		expectedOutput  []byte
		expectedMatched bool
	}{
		{
			name: "redact settings with email",
			rules: []analyzer.Rule{
				{
					Name:    "email-rule",
					Matcher: newEmailMatcher(ctrl),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED_EMAIL]",
						},
					},
				},
			},
			input:           []byte("Contact us at test@example.com for support"),
			expectedOutput:  []byte("Contact us at [REDACTED_EMAIL] for support"),
			expectedMatched: true,
		},
		{
			name: "redact settings with two entities",
			rules: []analyzer.Rule{
				{
					Name:    "email-rule",
					Matcher: newEmailMatcher(ctrl),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED_EMAIL]",
						},
					},
				},
				{
					Name:    "phone-rule",
					Matcher: newPhoneMatcher(ctrl),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED_PHONE]",
						},
					},
				},
			},
			input:           []byte("Contact us at test@example.com for support or call 1234567890 to talk to our team"),
			expectedOutput:  []byte("Contact us at [REDACTED_EMAIL] for support or call [REDACTED_PHONE] to talk to our team"),
			expectedMatched: true,
		},
		{
			name: "mask settings with credit card",
			rules: []analyzer.Rule{
				{
					Name: "card-rule",
					Matcher: func() *analyzermock.MockMatcher {
						mock := analyzermock.NewMockMatcher(ctrl)
						mock.EXPECT().Entity().Return(pattern.EntityCreditCard).AnyTimes()
						mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
							return bytes.Equal(input, []byte("1234567890123456"))
						}).AnyTimes()
						return mock
					}(),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.MASK,
						Mask: &analyzer.MaskSettings{
							MaskingChar: "*",
							MaxSize:     4,
						},
					},
				},
			},
			input:           []byte("Card number: 1234567890123456"),
			expectedOutput:  []byte("Card number: ****567890123456"),
			expectedMatched: true,
		},
		{
			name: "mask settings with zero MaxWorkers",
			rules: []analyzer.Rule{
				{
					Name: "card-rule-zero-mask",
					Matcher: func() *analyzermock.MockMatcher {
						mock := analyzermock.NewMockMatcher(ctrl)
						mock.EXPECT().Entity().Return(pattern.EntityCreditCard).AnyTimes()
						mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
							return bytes.Equal(input, []byte("1234567890123456"))
						}).AnyTimes()
						return mock
					}(),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.MASK,
						Mask: &analyzer.MaskSettings{
							MaskingChar: "*",
							MaxSize:     0, // This will trigger the n <= 0 condition
						},
					},
				},
			},
			input:           []byte("Card number: 1234567890123456"),
			expectedOutput:  []byte("Card number: *234567890123456"), // Should mask 1 character when MaxWorkers is 0
			expectedMatched: true,
		},
		{
			name: "no matches",
			rules: []analyzer.Rule{
				{
					Name: "email-rule",
					Matcher: func() *analyzermock.MockMatcher {
						mock := analyzermock.NewMockMatcher(ctrl)
						mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
						mock.EXPECT().Match(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
						return mock
					}(),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED]",
						},
					},
				},
			},
			input:           []byte("Just normal text here"),
			expectedOutput:  []byte("Just normal text here"),
			expectedMatched: false,
		},
		{
			name: "disabled rule",
			rules: []analyzer.Rule{
				{
					Name:    "disabled-rule",
					Disable: true,
					Matcher: newEmailMatcher(ctrl),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED]",
						},
					},
				},
			},
			input:           []byte("test@example.com"),
			expectedOutput:  []byte("test@example.com"),
			expectedMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
			require.NoError(t, err)
			ctx := context.Background()

			output := &bytes.Buffer{}
			details := ba.Anonymize(ctx, tt.rules, output, tt.input)

			assert.Equal(t, tt.expectedMatched, details.HasFindings)
			assert.Equal(t, tt.expectedOutput, output.Bytes())
		})
	}
}

func TestByteAnalyzer_AnonymizeWithTokenization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test that tokenization works correctly with various delimiters
	emailMatcher := newEmailMatcher(ctrl)
	rules := []analyzer.Rule{
		{
			Name:    "email-rule",
			Matcher: emailMatcher,
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact: &analyzer.RedactSettings{
					Placeholder: "[REDACTED]",
				},
			},
		},
	}

	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "comma delimiter",
			input:    []byte("email1@test.com, email2@test.com"),
			expected: []byte("[REDACTED], [REDACTED]"),
		},
		{
			name:     "parentheses delimiter",
			input:    []byte("Contact (user@test.com) for info"),
			expected: []byte("Contact ([REDACTED]) for info"),
		},
		{
			name:     "newline delimiter",
			input:    []byte("Line1\nuser@test.com\nLine3"),
			expected: []byte("Line1\n[REDACTED]\nLine3"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
			require.NoError(t, err)
			ctx := context.Background()

			output := &bytes.Buffer{}
			details := ba.Anonymize(ctx, rules, output, tt.input)

			assert.True(t, details.HasFindings)
			assert.Equal(t, tt.expected, output.Bytes())
		})
	}
}

func TestByteAnalyzer_EdgeCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("empty byte array", func(t *testing.T) {
		mockMatcher := analyzermock.NewMockMatcher(ctrl)
		mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
		mockMatcher.EXPECT().Match(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

		rules := []analyzer.Rule{
			{
				Name:    "test-rule",
				Matcher: mockMatcher,
			},
		}
		ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
		require.NoError(t, err)
		ctx := context.Background()

		// Test Anonymize with empty input
		output := &bytes.Buffer{}
		details := ba.Anonymize(ctx, rules, output, []byte(""))
		assert.False(t, details.HasFindings)
		// bytes.Buffer returns nil for empty buffer, so we compare with the empty input
		assert.Equal(t, 0, output.Len())
	})

	t.Run("large input handling", func(t *testing.T) {
		mockMatcher := analyzermock.NewMockMatcher(ctrl)
		mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
		mockMatcher.EXPECT().Match(gomock.Any(), gomock.Any()).Return(false).AnyTimes()

		rules := []analyzer.Rule{
			{
				Name:    "test-rule",
				Matcher: mockMatcher,
			},
		}
		ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
		require.NoError(t, err)
		ctx := context.Background()

		// Create a large input
		largeInput := make([]byte, 10000)
		for i := range largeInput {
			largeInput[i] = 'a'
		}

		// Should handle large inputs without issues
		output := &bytes.Buffer{}
		details := ba.Anonymize(ctx, rules, output, largeInput)
		assert.False(t, details.HasFindings)
		assert.Equal(t, largeInput, output.Bytes())
	})
}

// Test helper functions behavior
func TestByteAnalyzer_HelperFunctions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("punctuation removal behavior", func(t *testing.T) {
		// Test that the analyzer handles punctuation correctly during anonymization
		mockMatcher := analyzermock.NewMockMatcher(ctrl)
		mockMatcher.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
		mockMatcher.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
			// Match email pattern including punctuation
			return bytes.Contains(input, []byte("test@example.com"))
		}).AnyTimes()

		rules := []analyzer.Rule{
			{
				Name:    "email-rule",
				Matcher: mockMatcher,
				Settings: analyzer.RuleSettings{
					Strategy: analyzer.REDACT,
					Redact: &analyzer.RedactSettings{
						Placeholder: "[EMAIL]",
					},
				},
			},
		}

		ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
		require.NoError(t, err)
		ctx := context.Background()

		// Test with punctuation
		input := []byte("Email: test@example.com.")
		output := &bytes.Buffer{}
		details := ba.Anonymize(ctx, rules, output, input)

		assert.True(t, details.HasFindings)
		assert.NotEmpty(t, output.Bytes())
	})

	t.Run("tokenization with escape characters", func(t *testing.T) {
		rules := []analyzer.Rule{
			{
				Name:    "test-rule",
				Matcher: newEmailMatcher(ctrl),
				Settings: analyzer.RuleSettings{
					Strategy: analyzer.REDACT,
					Redact: &analyzer.RedactSettings{
						Placeholder: "[REDACTED]",
					},
				},
			},
		}

		ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
		require.NoError(t, err)
		ctx := context.Background()

		// Test with escape characters - use actual escape sequences instead of literal backslashes
		input := []byte("Line with\nemail@test.com\tand\rmore")
		output := &bytes.Buffer{}
		details := ba.Anonymize(ctx, rules, output, input)

		assert.True(t, details.HasFindings)
		assert.NotEmpty(t, output.Bytes())
	})
}

// TestByteAnalyzer_ConcurrentTokenProcessingParity verifies that concurrent token
// processing produces byte-for-byte identical output to the sequential path.
func TestByteAnalyzer_ConcurrentTokenProcessingParity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name  string
		rules func() []analyzer.Rule
		input []byte
	}{
		{
			name: "single email redacted",
			rules: func() []analyzer.Rule {
				return []analyzer.Rule{{
					Name:    "email-rule",
					Matcher: newEmailMatcher(ctrl),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact:   &analyzer.RedactSettings{Placeholder: "[REDACTED_EMAIL]"},
					},
				}}
			},
			input: []byte("Contact us at test@example.com for support"),
		},
		{
			name: "multiple tokens in order",
			rules: func() []analyzer.Rule {
				return []analyzer.Rule{{
					Name:    "email-rule",
					Matcher: newEmailMatcher(ctrl),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact:   &analyzer.RedactSettings{Placeholder: "[EMAIL]"},
					},
				}}
			},
			input: []byte("a@b.com x@y.com z@w.com d@e.com f@g.com"),
		},
		{
			name: "mask strategy output parity",
			rules: func() []analyzer.Rule {
				m := analyzermock.NewMockMatcher(ctrl)
				m.EXPECT().Entity().Return(pattern.EntityCreditCard).AnyTimes()
				m.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
					return bytes.Equal(input, []byte("1234567890123456"))
				}).AnyTimes()
				return []analyzer.Rule{{
					Name:    "card-rule",
					Matcher: m,
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.MASK,
						Mask:     &analyzer.MaskSettings{MaskingChar: "*", MaxSize: 4},
					},
				}}
			},
			input: []byte("Card number: 1234567890123456"),
		},
		{
			name: "no matches pass-through",
			rules: func() []analyzer.Rule {
				m := analyzermock.NewMockMatcher(ctrl)
				m.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
				m.EXPECT().Match(gomock.Any(), gomock.Any()).Return(false).AnyTimes()
				return []analyzer.Rule{{
					Name:    "no-match-rule",
					Matcher: m,
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact:   &analyzer.RedactSettings{Placeholder: "[X]"},
					},
				}}
			},
			input: []byte("Nothing sensitive here at all"),
		},
		{
			name: "exception honored in concurrent path",
			rules: func() []analyzer.Rule {
				return []analyzer.Rule{{
					Name:    "email-rule",
					Matcher: newEmailMatcher(ctrl),
					Exceptions: []analyzer.Exception{{
						Reason:  "Whitelisted",
						Matcher: newExceptionMatcher(ctrl, "safe@company.com"),
					}},
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact:   &analyzer.RedactSettings{Placeholder: "[REDACTED]"},
					},
				}}
			},
			input: []byte("safe@company.com and user@external.com"),
		},
		{
			name: "mixed strategies across tokens",
			rules: func() []analyzer.Rule {
				return []analyzer.Rule{
					{
						Name:    "email-rule",
						Matcher: newEmailMatcher(ctrl),
						Settings: analyzer.RuleSettings{
							Strategy: analyzer.REDACT,
							Redact:   &analyzer.RedactSettings{Placeholder: "[EMAIL]"},
						},
					},
					{
						Name:    "phone-rule",
						Matcher: newPhoneMatcher(ctrl),
						Settings: analyzer.RuleSettings{
							Strategy: analyzer.REDACT,
							Redact:   &analyzer.RedactSettings{Placeholder: "[PHONE]"},
						},
					},
				}
			},
			input: []byte("user@test.com 1234567890"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			rules := tt.rules()

			// Sequential path
			seqBA, err := analyzer.MakeByteAnalyzer(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
			require.NoError(t, err)
			seqOut := &bytes.Buffer{}
			seqDetails := seqBA.Anonymize(ctx, rules, seqOut, tt.input)

			// Concurrent token processing path
			concBA, err := analyzer.MakeByteAnalyzer(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{
				Concurrency: analyzer.ConcurrencyOptions{
					Enabled:                   true,
					ConcurrentTokenProcessing: true,
					TokenPoolSize:             4,
				},
			})
			require.NoError(t, err)
			concOut := &bytes.Buffer{}
			concDetails := concBA.Anonymize(ctx, rules, concOut, tt.input)

			assert.Equal(t, seqOut.Bytes(), concOut.Bytes(), "output must be byte-for-byte identical")
			assert.Equal(t, seqDetails.HasFindings, concDetails.HasFindings, "HasFindings must match")
		})
	}
}

func TestByteAnalyzer_AnonymizeExceptions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name            string
		rules           []analyzer.Rule
		input           []byte
		expectedOutput  []byte
		expectedMatched bool
	}{
		{
			name: "exception prevents redaction",
			rules: []analyzer.Rule{
				{
					Name:    "email-rule",
					Matcher: newEmailMatcher(ctrl),
					Exceptions: []analyzer.Exception{
						{
							Reason:  "Whitelisted email",
							Matcher: newExceptionMatcher(ctrl, "safe@company.com"),
						},
					},
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED_EMAIL]",
						},
					},
				},
			},
			input:           []byte("Contact safe@company.com for support"),
			expectedOutput:  []byte("Contact safe@company.com for support"),
			expectedMatched: false,
		},
		{
			name: "exception prevents masking",
			rules: []analyzer.Rule{
				{
					Name:    "phone-rule",
					Matcher: newPhoneMatcher(ctrl),
					Exceptions: []analyzer.Exception{
						{
							Reason:  "Emergency number",
							Matcher: newExceptionMatcher(ctrl, "911"),
						},
					},
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.MASK,
						Mask: &analyzer.MaskSettings{
							MaskingChar: "*",
							MaxSize:     2,
						},
					},
				},
			},
			input:           []byte("Call 911 for emergency"),
			expectedOutput:  []byte("Call 911 for emergency"),
			expectedMatched: false,
		},
		{
			name: "mixed content with exceptions and matches",
			rules: []analyzer.Rule{
				{
					Name:    "email-rule",
					Matcher: newEmailMatcher(ctrl),
					Exceptions: []analyzer.Exception{
						{
							Reason:  "Internal email",
							Matcher: newExceptionMatcher(ctrl, "internal@company.com"),
						},
					},
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED]",
						},
					},
				},
			},
			input:           []byte("Contact internal@company.com or external@hacker.com"),
			expectedOutput:  []byte("Contact internal@company.com or [REDACTED]"),
			expectedMatched: true,
		},
		{
			name: "multiple exceptions in same text",
			rules: []analyzer.Rule{
				{
					Name:    "email-rule",
					Matcher: newEmailMatcher(ctrl),
					Exceptions: []analyzer.Exception{
						{
							Reason:  "Support email",
							Matcher: newExceptionMatcher(ctrl, "support@company.com"),
						},
						{
							Reason:  "Admin email",
							Matcher: newExceptionMatcher(ctrl, "admin@company.com"),
						},
					},
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED]",
						},
					},
				},
			},
			input:           []byte("Emails: support@company.com, admin@company.com, user@external.com"),
			expectedOutput:  []byte("Emails: support@company.com, admin@company.com, [REDACTED]"),
			expectedMatched: true,
		},
		{
			name: "exception with punctuation handling",
			rules: []analyzer.Rule{
				{
					Name:    "email-rule",
					Matcher: newEmailMatcher(ctrl),
					Exceptions: []analyzer.Exception{
						{
							Reason:  "Safe email with punctuation",
							Matcher: newExceptionMatcher(ctrl, "safe@company.com."), // Match including punctuation
						},
					},
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED]",
						},
					},
				},
			},
			input:           []byte("Email: safe@company.com."),
			expectedOutput:  []byte("Email: safe@company.com."),
			expectedMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
			require.NoError(t, err)
			ctx := context.Background()

			output := &bytes.Buffer{}
			details := ba.Anonymize(ctx, tt.rules, output, tt.input)

			assert.Equal(t, tt.expectedMatched, details.HasFindings)
			assert.Equal(t, tt.expectedOutput, output.Bytes())
		})
	}
}

func TestByteAnalyzer_AnonymizeNilInput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	rules := []analyzer.Rule{
		{
			Name:    "email-rule",
			Matcher: newEmailMatcher(ctrl),
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact:   &analyzer.RedactSettings{Placeholder: "[REDACTED]"},
			},
		},
	}

	ctx := context.Background()
	output := &bytes.Buffer{}
	details := ba.Anonymize(ctx, rules, output, nil)

	assert.False(t, details.HasFindings)
	assert.Equal(t, 0, output.Len(), "output should be empty for nil input")
}

// TestByteAnalyzer_AnonymizeConcurrentCalls verifies that ByteAnalyzer.Anonymize
// can be called from many goroutines simultaneously with different inputs without
// data races or corrupted output.
func TestByteAnalyzer_AnonymizeConcurrentCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	rules := []analyzer.Rule{{
		Name:    "email-rule",
		Matcher: newEmailMatcher(ctrl),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact:   &analyzer.RedactSettings{Placeholder: "[REDACTED]"},
		},
	}}

	ctx := context.Background()

	const goroutines = 50
	results := make([]struct {
		output     *bytes.Buffer
		hasFinding bool
	}, goroutines)

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			input := []byte(fmt.Sprintf("email%d@test.com", idx))
			out := &bytes.Buffer{}
			details := ba.Anonymize(ctx, rules, out, input)
			results[idx].output = out
			results[idx].hasFinding = details.HasFindings
		}(i)
	}

	wg.Wait()

	for i := range goroutines {
		assert.True(t, results[i].hasFinding, "goroutine %d should find a match", i)
		assert.Equal(t, "[REDACTED]", results[i].output.String(), "goroutine %d output mismatch", i)
	}
}

// TestByteAnalyzer_AnonymizeConcurrentMixedInputs verifies concurrent calls with
// mixed matching and non-matching inputs produce correct independent results.
func TestByteAnalyzer_AnonymizeConcurrentMixedInputs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	emailMock := analyzermock.NewMockMatcher(ctrl)
	emailMock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	emailMock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
		return bytes.Contains(input, []byte("@")) && bytes.Contains(input, []byte("."))
	}).AnyTimes()

	phoneMock := analyzermock.NewMockMatcher(ctrl)
	phoneMock.EXPECT().Entity().Return(pattern.EntityPhone).AnyTimes()
	phoneMock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
		return len(input) >= 10 && bytes.ContainsAny(input, "0123456789")
	}).AnyTimes()

	ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	ctx := context.Background()

	// Inputs: matching email, matching phone, non-sensitive plain text, mixed content
	inputs := []struct {
		input    []byte
		expected bool
	}{
		{[]byte("user@test.com"), true},
		{[]byte("plain text here"), false},
		{[]byte("1234567890 call us"), true},
		{[]byte("no match at all"), false},
		{[]byte("test@example.com more info"), true},
		{[]byte("just words"), false},
		{[]byte("9876543210 for help"), true},
		{[]byte("hello world"), false},
	}

	results := make([]bool, len(inputs))
	outputs := make([]*bytes.Buffer, len(inputs))

	var wg sync.WaitGroup
	for i, inp := range inputs {
		wg.Add(1)
		go func(idx int, input []byte, expectMatch bool) {
			defer wg.Done()

			rules := []analyzer.Rule{
				{Name: "email", Matcher: emailMock, Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[EMAIL]"}}},
				{Name: "phone", Matcher: phoneMock, Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[PHONE]"}}},
			}

			out := &bytes.Buffer{}
			details := ba.Anonymize(ctx, rules, out, input)
			results[idx] = details.HasFindings
			outputs[idx] = out
		}(i, inp.input, inp.expected)
	}

	wg.Wait()

	for i, inp := range inputs {
		assert.Equal(t, inp.expected, results[i], "input %q at index %d: HasFindings mismatch", inp.input, i)
		if inp.expected {
			assert.NotEqual(t, string(inp.input), outputs[i].String(), "input %q at index %d: output should be anonymized", inp.input, i)
		} else {
			assert.Equal(t, string(inp.input), outputs[i].String(), "input %q at index %d: output should be unchanged", inp.input, i)
		}
	}
}

// TestByteAnalyzer_StopConcurrentWithAnonymize verifies that calling Stop
// concurrently with Anonymize does not panic or cause races.
func TestByteAnalyzer_StopConcurrentWithAnonymize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	rules := []analyzer.Rule{{
		Name:    "rule",
		Matcher: newEmailMatcher(ctrl),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact:   &analyzer.RedactSettings{Placeholder: "[X]"},
		},
	}}

	ctx := context.Background()

	var wg sync.WaitGroup

	// Spawn goroutines to call Anonymize.
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out := &bytes.Buffer{}
			_ = ba.Anonymize(ctx, rules, out, []byte("a@b.c"))
		}()
	}

	// Call Stop concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		ba.Stop()
	}()

	wg.Wait()
}

// TestByteAnalyzer_AnonymizeConcurrentTokenProcessing verifies that the concurrent
// token processing path also handles concurrent calls safely.
func TestByteAnalyzer_AnonymizeConcurrentTokenProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{
		Concurrency: analyzer.ConcurrencyOptions{
			Enabled:                   true,
			ConcurrentTokenProcessing: true,
			TokenPoolSize:             4,
		},
	})
	require.NoError(t, err)

	rules := []analyzer.Rule{{
		Name:    "email-rule",
		Matcher: newEmailMatcher(ctrl),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact:   &analyzer.RedactSettings{Placeholder: "[R]"},
		},
	}}

	ctx := context.Background()

	const goroutines = 30
	results := make([]bool, goroutines)
	outputs := make([]*bytes.Buffer, goroutines)

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			input := []byte(fmt.Sprintf("email%d@test.com text%d", idx, idx))
			out := &bytes.Buffer{}
			details := ba.Anonymize(ctx, rules, out, input)
			results[idx] = details.HasFindings
			outputs[idx] = out
		}(i)
	}

	wg.Wait()

	for i := range goroutines {
		assert.True(t, results[i], "goroutine %d: expected match", i)
		assert.Contains(t, outputs[i].String(), "[R]", "goroutine %d: expected redaction in output", i)
	}
}

func BenchmarkByteAnalyzerAnonymize(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := analyzer.NewSerialRulesRuner(logger, analyzercache.NewNoopRuleMatchingCache())
	ba := analyzer.NewByteAnalyzer(logger, runner)

	// Realistic input containing various PII types
	input := []byte("Contact user@example.com or call +1-555-123-4567. " +
		"Credit card: 4111111111111111. CPF: 529.982.247-25. CNPJ: 11.222.333/0001-81. " +
		"IP: 192.168.1.100. Visit https://www.example.com/page. " +
		"SSN: 123-45-6789. VIN: 1HGCM82633A004352. " +
		"UUID: 550e8400-e29b-41d4-a716-446655440000. " +
		"Another email: support@company.org. IP: 10.0.0.1.")

	rules := []analyzer.Rule{
		{Name: "email", Matcher: pattern.EmailMatcher(), Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[EMAIL]"}}},
		{Name: "creditcard", Matcher: pattern.CreditCardMatcher(), Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[CC]"}}},
		{Name: "cpf", Matcher: pattern.CPFMatcher(), Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[CPF]"}}},
		{Name: "cnpj", Matcher: pattern.CNPJMatcher(), Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[CNPJ]"}}},
		{Name: "ip", Matcher: pattern.IPMatcher(), Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[IP]"}}},
		{Name: "ssn", Matcher: pattern.SSNMatcher(), Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[SSN]"}}},
	}

	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		var output bytes.Buffer
		_ = ba.Anonymize(ctx, rules, &output, input)
	}
}

func BenchmarkByteAnalyzerAnonymizeLargeInput(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := analyzer.NewSerialRulesRuner(logger, analyzercache.NewNoopRuleMatchingCache())
	ba := analyzer.NewByteAnalyzer(logger, runner)

	// Build a ~5KB input with repeated PII patterns
	base := "Contact user@example.com or call +1-555-123-4567. Credit card: 4111111111111111. CPF: 529.982.247-25. "
	largeInput := []byte("")
	for range 50 {
		largeInput = append(largeInput, []byte(base)...)
	}

	rules := []analyzer.Rule{
		{Name: "email", Matcher: pattern.EmailMatcher(), Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[EMAIL]"}}},
		{Name: "creditcard", Matcher: pattern.CreditCardMatcher(), Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[CC]"}}},
		{Name: "cpf", Matcher: pattern.CPFMatcher(), Settings: analyzer.RuleSettings{Strategy: analyzer.REDACT, Redact: &analyzer.RedactSettings{Placeholder: "[CPF]"}}},
	}

	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		var output bytes.Buffer
		_ = ba.Anonymize(ctx, rules, &output, largeInput)
	}
}

func TestByteAnalyzer_AnonymizeEmptyRules(t *testing.T) {
	ba, err := analyzer.MakeByteAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	ctx := context.Background()

	// Empty rules should not panic and should return empty output
	input := []byte("test@example.com sensitive data here")
	output := &bytes.Buffer{}
	details := ba.Anonymize(ctx, []analyzer.Rule{}, output, input)

	assert.False(t, details.HasFindings)
	assert.Equal(t, 0, output.Len(), "output should be empty when rules are empty")

	// Nil rules slice should also not panic
	output2 := &bytes.Buffer{}
	details2 := ba.Anonymize(ctx, nil, output2, input)

	assert.False(t, details2.HasFindings)
	assert.Equal(t, 0, output2.Len(), "output should be empty when rules are nil")
}
