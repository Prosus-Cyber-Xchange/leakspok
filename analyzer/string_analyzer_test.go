package analyzer_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/ifood/leakspok/analyzer"
	analyzermock "github.com/ifood/leakspok/analyzer/mocks"
	"github.com/ifood/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Helper function to create a redacting matcher
func newRedactingMatcher(ctrl *gomock.Controller) *analyzermock.MockMatcher {
	mock := analyzermock.NewMockMatcher(ctrl)
	mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
	mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
		return string(input) == "secret123"
	}).AnyTimes()
	return mock
}

func TestNewStringAnalyzer(t *testing.T) {
	sa := analyzer.NewStringAnalyzer(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	// StringAnalyzer should be properly initialized
	assert.NotNil(t, sa)
}

func TestStringAnalyzer_Anonymize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name            string
		rules           []analyzer.Rule
		input           string
		expectedOutput  string
		expectedMatched bool
	}{
		{
			name:            "empty string",
			rules:           []analyzer.Rule{},
			input:           "",
			expectedOutput:  "",
			expectedMatched: false,
		},
		{
			name: "string with sensitive data to redact",
			rules: []analyzer.Rule{
				{
					Name:    "secret-rule",
					Matcher: newRedactingMatcher(ctrl),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "[REDACTED]",
						},
					},
				},
			},
			input:           "The password is secret123 for access",
			expectedOutput:  "The password is [REDACTED] for access",
			expectedMatched: true,
		},
		{
			name: "string with no sensitive data",
			rules: []analyzer.Rule{
				{
					Name:    "secret-rule",
					Matcher: newRedactingMatcher(ctrl),
				},
			},
			input:           "This is just normal text without secrets",
			expectedOutput:  "This is just normal text without secrets",
			expectedMatched: false,
		},
		{
			name: "string with unicode characters",
			rules: []analyzer.Rule{
				{
					Name: "unicode-secret-rule",
					Matcher: func() *analyzermock.MockMatcher {
						mock := analyzermock.NewMockMatcher(ctrl)
						mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
						mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
							return string(input) == "秘密123"
						}).AnyTimes()
						return mock
					}(),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "***",
						},
					},
				},
			},
			input:           "用户密码是 秘密123 请保密",
			expectedOutput:  "用户密码是 *** 请保密",
			expectedMatched: true,
		},
		{
			name: "long string performance test",
			rules: []analyzer.Rule{
				{
					Name: "performance-rule",
					Matcher: func() *analyzermock.MockMatcher {
						mock := analyzermock.NewMockMatcher(ctrl)
						mock.EXPECT().Entity().Return(pattern.EntityEmail).AnyTimes()
						mock.EXPECT().Match(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input []byte) bool {
							return string(input) == "findme"
						}).AnyTimes()
						return mock
					}(),
					Settings: analyzer.RuleSettings{
						Strategy: analyzer.REDACT,
						Redact: &analyzer.RedactSettings{
							Placeholder: "X",
						},
					},
				},
			},
			input:           "This is a very long string with lots of content that should be processed efficiently. The secret word is findme and it should be found quickly.",
			expectedOutput:  "This is a very long string with lots of content that should be processed efficiently. The secret word is X and it should be found quickly.",
			expectedMatched: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa, err := analyzer.MakeStringAnalyzer(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
			require.NoError(t, err)
			ctx := context.Background()

			result, details := sa.Anonymize(ctx, tt.rules, tt.input)

			assert.Equal(t, tt.expectedOutput, result)
			assert.Equal(t, tt.expectedMatched, details.HasFindings)
		})
	}
}
