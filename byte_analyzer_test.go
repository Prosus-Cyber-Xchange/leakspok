package leakspok_test

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ifood/leakspok/analyzer"
	"github.com/ifood/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactCPF(t *testing.T) {
	placeholder := "[CPF_REDACTED]"
	cpfRule := analyzer.Rule{
		Name:        "brazilian_cpf",
		Description: "Brazilian CPF",
		Matcher:     pattern.CPFMatcher(),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact: &analyzer.RedactSettings{
				Placeholder: placeholder,
			},
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
		isLeak   bool
	}{
		{
			name:     "CPF with newline prefix",
			input:    `" \n216.187.210-97"`,
			expected: `" \n` + placeholder + `"`,
			isLeak:   true,
		},
		{
			name:     "CPF with carriage return prefix",
			input:    `" \r216.187.210-97"`,
			expected: `" \r` + placeholder + `"`,
			isLeak:   true,
		},
		{
			name:     "CPF with tab prefix",
			input:    `" \t216.187.210-97"`,
			expected: `" \t` + placeholder + `"`,
			isLeak:   true,
		},
		{
			name:     "CPF with newlines around it",
			input:    `" \n216.187.210-97\n"`,
			expected: `" \n` + placeholder + `\n"`,
			isLeak:   true,
		},
		{
			name:     "CPF with escaped characters",
			input:    `" \\216.187.210-97\\\"\\n\"}]"`,
			expected: `" \\` + placeholder + `\\\"\\n\"}]"`,
			isLeak:   true,
		},
		{
			name:  "CPF in JSON model content (double quote variant 1)",
			input: `"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\\"216.187.210-97\\\"\\n\"}], \"temperature\": 0.7}"`,
			expected: `"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\\"` +
				placeholder + `\\\"\\n\"}], \"temperature\": 0.7}"`,
			isLeak: true,
		},
		{
			name:  "CPF in JSON model content (double quote variant 2)",
			input: `"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\\"216.187.210-97\\\"\\n\"}], \"temperature\": 0.7}"`,
			expected: `"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\\"` +
				placeholder + `\\\"\\n\"}], \"temperature\": 0.7}"`,
			isLeak: true,
		},
		{
			name:     "CPF in file path with quoted segments",
			input:    `"Unable to access the notebook \"/Workspace/Users \"216.187.210-97\" stocks_information"`,
			expected: `"Unable to access the notebook \"/Workspace/Users \"` + placeholder + `\" stocks_information"`,
			isLeak:   true,
		},
		{
			name:     "CPF in file path as directory segment",
			input:    `"Unable to access the notebook \"/Workspace/Users/216.187.210-97/stocks_information"`,
			expected: `"Unable to access the notebook \"/Workspace/Users/` + placeholder + `/stocks_information"`,
			isLeak:   true,
		},
		{
			name:     "CPF in file path with quotes around CPF",
			input:    `"Unable to access the notebook \"/Workspace/Users/\"216.187.210-97\"/stocks_information"`,
			expected: `"Unable to access the notebook \"/Workspace/Users/\"` + placeholder + `\"/stocks_information"`,
			isLeak:   true,
		},
		{
			name: "CPF in multi-line JSON with single quotes",
			input: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking '111444777-35" abc"}],
                 "temperature": 0.1}'`,
			expected: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking '` + placeholder + `" abc"}],
                 "temperature": 0.1}'`,
			isLeak: true,
		},
		{
			name: "CPF surrounded by commas",
			input: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking ,,,,111444777-35,,,,, abc"}],
                 "temperature": 0.1}'`,
			expected: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking ,,,,` + placeholder + `,,,,, abc"}],
                 "temperature": 0.1}'`,
			isLeak: true,
		},
		{
			name: "CPF at end of JSON content",
			input: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking ,,,,111444777-35"}],
                 "temperature": 0.1}'`,
			expected: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking ,,,,` + placeholder + `"}],
                 "temperature": 0.1}'`,
			isLeak: true,
		},
		{
			name: "CPF ending with quote",
			input: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking ,,,,111444777-35'"],
                 "temperature": 0.1}'`,
			expected: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking ,,,,` + placeholder + `'"],
                 "temperature": 0.1}'`,
			isLeak: true,
		},
		{
			name: "Invalid CPF (extra digit - not redacted)",
			input: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking ,,,,111444777-351,,,,, abc"}],
                 "temperature": 0.1}'`,
			expected: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking ,,,,111444777-351,,,,, abc"}],
                 "temperature": 0.1}'`,
			isLeak: false,
		},
		{
			name: "CPF with out-of-order JSON structure",
			input: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"content": "Say this: my cpf is 383.413.710-30"}"role": "user"}],
                 "temperature": 0.1}'`,
			expected: `'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"content": "Say this: my cpf is ` + placeholder + `"}"role": "user"}],
                 "temperature": 0.1}'`,
			isLeak: true,
		},
	}

	rulesForAnonymize := []analyzer.Rule{cpfRule}
	byteAnalyzer, err := analyzer.MakeByteAnalyzer(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			_ = byteAnalyzer.Anonymize(t.Context(), rulesForAnonymize, output, []byte(test.input))
			gotStr := output.String()

			if test.isLeak { //nolint:nestif // table-driven test: nested ifs are idiomatic here
				// If cpf is in the string, it wasn't masked
				if gotStr != test.expected {
					t.Errorf("For input %q, expected %v, got %v", test.input, test.expected, gotStr)
				}

				// If REDACT string is not in the string, it wasn't masked
				if !strings.Contains(gotStr, placeholder) {
					t.Errorf("For input %q, expected %v, got %v", test.input, placeholder, gotStr)
				}
			} else {
				// Not leaked: the potential leak is still in the string (as expected)
				if gotStr != test.expected {
					t.Errorf("For input %q, expected %v, got %v", test.input, test.expected, gotStr)
				}

				// Not leaked: the REDACT string is in the string (not expected)
				if strings.Contains(gotStr, placeholder) {
					t.Errorf("For input %q, expected %v, got %v", test.input, placeholder, gotStr)
				}
			}
		})
	}
}

// TestRedactEmail tests the redaction of email addresses
func TestRedactEmail(t *testing.T) {
	placeholder := "[EMAIL_REDACTED]"
	emailRule := analyzer.Rule{
		Name:        "email_address",
		Description: "Email Address",
		Matcher:     pattern.EmailMatcher(),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact: &analyzer.RedactSettings{
				Placeholder: placeholder,
			},
		},
	}

	tests := []struct {
		input    string
		expected string
		isLeak   bool
	}{
		{`" \nandre.zina@gmail.com"`,
			`" \n` + placeholder + `"`,
			true},
		{`"webpack-dev-server@4.15.2"`,
			`"webpack-dev-server@4.15.2"`,
			false},
		{`" \nandre.zina@1.2.3"`,
			`" \nandre.zina@1.2.3"`,
			false},
		{`" \randre.zina@gmail.com"`,
			`" \r` + placeholder + `"`,
			true},
		{`" \tandre.zina@gmail.com"`,
			`" \t` + placeholder + `"`,
			true},
		{`" \nandre.zina@gmail.com\n"`,
			`" \n` + placeholder + `\n"`,
			true},
		{`" \\jhon.doe@gmail.com\\\"\\n\"}]"`,
			`" \\` + placeholder + `\\\"\\n\"}]"`,
			true},
		{`"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\"jhon.doe@gmail.com\\\"\\n\"}], \"temperature\": 0.7}"`,
			`"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\"` +
				placeholder + `\\\"\\n\"}], \"temperature\": 0.7}"`,
			true},
		{`"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\\"jhon.doe@gmail.com\\\"\\n\"}], \"temperature\": 0.7}"`,
			`"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\\"` +
				placeholder + `\\\"\\n\"}], \"temperature\": 0.7}"`,
			true},

		// Cover this case is a tradeoff. If we make `/` a token delimiter, then CNPJ matching breaks
		// since CNPJ have the format `11.444.777/0001-61`.
		{`"Unable to access the notebook "/Workspace/Users "jhon.doe@gmail.com" stocks_information"`,
			`"Unable to access the notebook "/Workspace/Users "` + placeholder + `" stocks_information"`,
			true},
		{`Unable to access the notebook "/Workspace/Users/"john.doe@ifood.com.br"/stocks_information"`,
			`Unable to access the notebook "/Workspace/Users/"` + placeholder +
				`"/stocks_information"`,
			true,
		},
		{`Unable to access the notebook "/Workspace/Users/john.doe@ifood.com.br/stocks_information"`,
			`Unable to access the notebook "/Workspace/Users/` + placeholder +
				`/stocks_information"`,
			true,
		},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking "joao.silva@gmail.com" aa"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking "` + placeholder + `" aa"}],
                 "temperature": 0.1}'`,
			true,
		},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking 'joao.silva@gmail.com"" abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking '` + placeholder + `"" abc"}],
                 "temperature": 0.1}'`,
			true,
		},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking joao.silva@gmail.com, abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking ` + placeholder + `, abc"}],
                 "temperature": 0.1}'`,
			true,
		},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking ,joao.silva@gmail.com,,,,!!!! abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking ,` + placeholder + `,,,,!!!! abc"}],
                 "temperature": 0.1}'`,
			true,
		},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking ,!!!;!?????joao@teste.br,,,,!!!!????? abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking ,!!!;!?????` + placeholder + `,,,,!!!!????? abc"}],
                 "temperature": 0.1}'`,
			true,
		},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking '}joao@teste.br"{} abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking '}` + placeholder + `"{} abc"}],
                 "temperature": 0.1}'`,
			true,
		},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking ,!!!;!?????joao@teste,,,,!!!!????? abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking ,!!!;!?????joao@teste,,,,!!!!????? abc"}],
                 "temperature": 0.1}'`,
			false,
		},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking,joao.test@gmail.com "}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking,` + placeholder + ` "}],
                 "temperature": 0.1}'`,
			true,
		},
	}

	emailRulesForAnonymize := []analyzer.Rule{emailRule}
	byteAnalyzer, err := analyzer.MakeByteAnalyzer(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	for _, test := range tests {
		output := &bytes.Buffer{}
		_ = byteAnalyzer.Anonymize(t.Context(), emailRulesForAnonymize, output, []byte(test.input))
		gotStr := output.String()

		if test.isLeak { //nolint:nestif // table-driven test: nested ifs are idiomatic here
			// If email is in the string, it wasn't masked
			if gotStr != test.expected {
				t.Errorf("For input %q, expected %v, got %v", test.input, test.expected, gotStr)
			}

			// If REDACT string is not in the string, it wasn't masked
			if !strings.Contains(gotStr, placeholder) {
				t.Errorf("For input %q, expected %v, got %v", test.input, placeholder, gotStr)
			}
		} else {
			// Not leaked: the potential leak is still in the string (as expected)
			if gotStr != test.expected {
				t.Errorf("For input %q, expected %v, got %v", test.input, test.expected, gotStr)
			}

			// Not leaked: the REDACT string is in the string (not expected)
			if strings.Contains(gotStr, placeholder) {
				t.Errorf("For input %q, expected %v, got %v", test.input, placeholder, gotStr)
			}
		}
	}
}

// TestRedactIPAddress tests the redaction of IP addresses
func TestRedactIPAddress(t *testing.T) {
	placeholder := "[IP_REDACTED]"
	ipRule := analyzer.Rule{
		Name:        "ip_address",
		Description: "IP Address",
		Matcher:     pattern.IPv4Matcher(),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.REDACT,
			Redact: &analyzer.RedactSettings{
				Placeholder: placeholder,
			},
		},
	}

	tests := []struct {
		input    string
		expected string
		isLeak   bool
	}{
		{`" \n180.112.90.22"`,
			`" \n` + placeholder + `"`,
			true},
		{`" \r180.112.90.22"`,
			`" \r` + placeholder + `"`,
			true},
		{`" \t180.112.90.22"`,
			`" \t` + placeholder + `"`,
			true},
		{`" \n180.112.90.22\n"`,
			`" \n` + placeholder + `\n"`,
			true},
		{`" \\180.112.90.22\\\"\\n\"}]"`,
			`" \\` + placeholder + `\\\"\\n\"}]"`,
			true},
		{`"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\"180.112.90.22\\\"\\n\"}], \"temperature\": 0.7}"`,
			`"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\"` +
				placeholder + `\\\"\\n\"}], \"temperature\": 0.7}"`,
			true},
		{`"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\\"180.112.90.22\\\"\\n\"}], \"temperature\": 0.7}"`,
			`"{\"model\": \"gpt-4o-mini\", \"messages\": [{\"role\": \"user\", \"content\": \"\\n\\\"` +
				placeholder + `\\\"\\n\"}], \"temperature\": 0.7}"`,
			true},
		{`"Unable to access the notebook "/Workspace/Users "180.112.90.22" stocks_information"`,
			`"Unable to access the notebook "/Workspace/Users "` + placeholder + `" stocks_information"`,
			true},
		{`Unable to access the notebook "/Workspace/Users/"180.112.90.22"/stocks_information"`,
			`Unable to access the notebook "/Workspace/Users/"` + placeholder +
				`"/stocks_information"`,
			true,
		},
		{`Unable to access the notebook "/Workspace/Users/180.112.90.22/stocks_information"`,
			`Unable to access the notebook "/Workspace/Users/` + placeholder +
				`/stocks_information"`,
			true,
		},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking 180.112.90.22 abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ` + placeholder + ` abc"}],
                 "temperature": 0.1}'`,
			true},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,180.112.90.22,, abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,` + placeholder + `,, abc"}],
                 "temperature": 0.1}'`,
			true},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,'180.112.90.22"{ abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,'` + placeholder + `"{ abc"}],
                 "temperature": 0.1}'`,
			true},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,????????;;180.112.90.22,????, abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,????????;;` + placeholder + `,????, abc"}],
                 "temperature": 0.1}'`,
			true},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,????????;;180.112.90.22"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,????????;;` + placeholder + `"}],
                 "temperature": 0.1}'`,
			true},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,????????;;999.112.90.999,::????, abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking ,????????;;999.112.90.999,::????, abc"}],
                 "temperature": 0.1}'`,
			false},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking,180.112.90.22 abc"}],
                 "temperature": 0.1}'`,
			`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking,` + placeholder + ` abc"}],
                 "temperature": 0.1}'`,
			true},
	}

	ipRulesForAnonymize := []analyzer.Rule{ipRule}
	byteAnalyzer, err := analyzer.MakeByteAnalyzer(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	for _, test := range tests {
		output := &bytes.Buffer{}
		_ = byteAnalyzer.Anonymize(t.Context(), ipRulesForAnonymize, output, []byte(test.input))
		gotStr := output.String()

		if test.isLeak { //nolint:nestif // table-driven test: nested ifs are idiomatic here
			// If IP is in the string, it wasn't masked
			if gotStr != test.expected {
				t.Errorf("For input %q expected not find %v in %v", test.input, test.expected, gotStr)
			}

			// If REDACT string is not in the string, it wasn't masked
			if !strings.Contains(gotStr, placeholder) {
				t.Errorf("For input %q expected find %v in %v", test.input, placeholder, gotStr)
			}
		} else {
			// Not leaked: the potential leak is still in the string (as expected)
			if gotStr != test.expected {
				t.Errorf("For input %q expected finding %v in %v", test.input, test.expected, gotStr)
			}

			// Not leaked: the REDACT string is in the string (not expected)
			if strings.Contains(gotStr, placeholder) {
				t.Errorf("For input %q, it is not expected find %v in %v", test.input, placeholder, gotStr)
			}
		}
	}
}

// TestMaskFindings tests the masking of findings
//
//gocyclo:ignore
//nolint:gocognit
func TestMaskFindings(t *testing.T) {
	cpfTests := []struct {
		input  string
		leak   string
		isLeak bool
	}{
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking 111444777-35 abc"}],
                 "temperature": 0.1}'`, "111444777-35", true},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing cpf leaking 111444777-34 abc"}],
                 "temperature": 0.1}'`, "111444777-34", false},
	}

	ipTests := []struct {
		input  string
		leak   string
		isLeak bool
	}{
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking 180.112.90.22 abc"}],
                 "temperature": 0.1}'`, "180.112.90.22", true},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking 1.1.1.1 abc"}],
                 "temperature": 0.1}'`, "1.1.1.1", true},
		{`'{
				"model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing IP address leaking 200.123.2.289 abc"}],
                 "temperature": 0.1}'`, "200.123.2.289", false},
	}

	emailTests := []struct {
		input  string
		leak   string
		isLeak bool
	}{
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking joao.silva@gmail.com abc"}],
                 "temperature": 0.1}'`, "joao.silva@gmail.com", true},
		{`'{
                 "model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking j@e.com abc"}],
                 "temperature": 0.1}'`, "j@e.com", true},
		{`'{
				"model": "gpt-3.5-turbo",
                 "messages": [{"role": "user", "content": "testing email leaking joao.silva@mail.com.br abc"}],
                 "temperature": 0.1}'`, "joao.silva@mail.com.br", true},
	}

	cpfRule := analyzer.Rule{
		Name:        "brazilian_CPF",
		Description: "Brazilian CPF",
		Matcher:     pattern.CPFMatcher(),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.MASK,
			Mask: &analyzer.MaskSettings{
				MaskingChar: "*",
				MaxSize:     3,
			},
		},
	}

	ipRule := analyzer.Rule{
		Name:        "ip_address",
		Description: "IP Addresses",
		Matcher:     pattern.IPMatcher(),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.MASK,
			Mask: &analyzer.MaskSettings{
				MaskingChar: "*",
				MaxSize:     3,
			},
		},
	}

	emailRule := analyzer.Rule{
		Name:        "email_address",
		Description: "valid email address",
		Matcher:     pattern.EmailMatcher(),
		Settings: analyzer.RuleSettings{
			Strategy: analyzer.MASK,
			Mask: &analyzer.MaskSettings{
				MaskingChar: "*",
				MaxSize:     3,
			},
		},
	}

	rules := []analyzer.Rule{
		cpfRule,
		ipRule,
		emailRule,
	}

	byteAnalyzer, err := analyzer.MakeByteAnalyzer(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(t, err)

	for _, test := range cpfTests {
		output := &bytes.Buffer{}
		_ = byteAnalyzer.Anonymize(t.Context(), rules, output, []byte(test.input))
		gotStr := output.String()

		if test.isLeak {
			if strings.Contains(gotStr, test.leak) {
				t.Errorf("For input %q expected not find %v in %v", test.input, test.leak, gotStr)
			}
		} else {
			if !strings.Contains(gotStr, test.leak) {
				t.Errorf("For input %q expected finding %v in %v", test.input, test.leak, gotStr)
			}
		}
	}

	for _, test := range ipTests {
		output := &bytes.Buffer{}
		_ = byteAnalyzer.Anonymize(t.Context(), rules, output, []byte(test.input))
		gotStr := output.String()

		if test.isLeak {
			if strings.Contains(gotStr, test.leak) {
				t.Errorf("For input %q expected not find %v in %v", test.input, test.leak, gotStr)
			}
		} else {
			if !strings.Contains(gotStr, test.leak) {
				t.Errorf("For input %q expected finding %v in %v", test.input, test.leak, gotStr)
			}
		}
	}

	for _, test := range emailTests {
		output := &bytes.Buffer{}
		_ = byteAnalyzer.Anonymize(t.Context(), rules, output, []byte(test.input))
		gotStr := output.String()

		if test.isLeak {
			if strings.Contains(gotStr, test.leak) {
				t.Errorf("For input %q expected not find %v in %v", test.input, test.leak, gotStr)
			}
		} else {
			if !strings.Contains(gotStr, test.leak) {
				t.Errorf("For input %q expected finding %v in %v", test.input, test.leak, gotStr)
			}
		}
	}
}

// It is expected that the CPNJ is not redacted
// See analyzer.TokenIterator documentation for more details
func TestRedactMultiLine(t *testing.T) {
	rules := []analyzer.Rule{
		{
			Name:        "email_address",
			Description: "Email Address",
			Matcher:     pattern.EmailMatcher(),
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact: &analyzer.RedactSettings{
					Placeholder: "[EMAIL_REDACTED]",
				},
			},
		},
		{
			Name:    "cpf_number",
			Matcher: pattern.CPFMatcher(),
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact: &analyzer.RedactSettings{
					Placeholder: "[CPF_REDACTED]",
				},
			},
		},
		{
			Name:    "cnpj_number",
			Matcher: pattern.CNPJMatcher(),
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact: &analyzer.RedactSettings{
					Placeholder: "[CNPJ_REDACTED]",
				},
			},
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{`Estamos trabalhando com a TechSolutions Brasil, uma empresa de consultoria em tecnologia especializada em infraestrutura em nuvem.

A empresa está registrada sob o CNPJ 23.557.022/0001-52 e está em operação desde 2018.

A pessoa de contato principal é João Silva, que possui CPF 759.066.110-45 e atua como CTO da organização.

Você pode contatá-lo em joao.silva@techsolutions.com.br para quaisquer dúvidas técnicas ou discussões sobre projetos.

Estamos monitorando a infraestrutura de rede deles e notamos alguns padrões incomuns nos logs do servidor.`,

			`Estamos trabalhando com a TechSolutions Brasil, uma empresa de consultoria em tecnologia especializada em infraestrutura em nuvem.

A empresa está registrada sob o CNPJ 23.557.022/0001-52 e está em operação desde 2018.

A pessoa de contato principal é João Silva, que possui CPF [CPF_REDACTED] e atua como CTO da organização.

Você pode contatá-lo em [EMAIL_REDACTED] para quaisquer dúvidas técnicas ou discussões sobre projetos.

Estamos monitorando a infraestrutura de rede deles e notamos alguns padrões incomuns nos logs do servidor.`,
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	byteAnalyzer, err := analyzer.MakeByteAnalyzer(t.Context(), logger, analyzer.RunnerOptions{})
	require.NoError(t, err)

	for _, test := range tests {
		output := &bytes.Buffer{}
		details := byteAnalyzer.Anonymize(t.Context(), rules, output, []byte(test.input))
		gotStr := output.String()

		assert.Equal(t, test.expected, gotStr)
		assert.True(t, details.HasFindings)
	}
}

// BenchmarkRedactMultiLine benchmarks the Anonymize operation on multi-line content
// containing email addresses, CPF, and CNPJ numbers
func BenchmarkRedactMultiLine(b *testing.B) {
	rules := []analyzer.Rule{
		{
			Name:        "email_address",
			Description: "Email Address",
			Matcher:     pattern.EmailMatcher(),
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact: &analyzer.RedactSettings{
					Placeholder: "[EMAIL_REDACTED]",
				},
			},
		},
		{
			Name:    "cpf_number",
			Matcher: pattern.CPFMatcher(),
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact: &analyzer.RedactSettings{
					Placeholder: "[CPF_REDACTED]",
				},
			},
		},
		{
			Name:    "cnpj_number",
			Matcher: pattern.CNPJMatcher(),
			Settings: analyzer.RuleSettings{
				Strategy: analyzer.REDACT,
				Redact: &analyzer.RedactSettings{
					Placeholder: "[CNPJ_REDACTED]",
				},
			},
		},
	}

	input := []byte(`Estamos trabalhando com a TechSolutions Brasil, uma empresa de consultoria em tecnologia especializada em infraestrutura em nuvem.

A empresa está registrada sob o CNPJ 23.557.022/0001-52 e está em operação desde 2018.

A pessoa de contato principal é João Silva, que possui CPF 759.066.110-45 e atua como CTO da organização.

Você pode contatá-lo em joao.silva@techsolutions.com.br para quaisquer dúvidas técnicas ou discussões sobre projetos.

Estamos monitorando a infraestrutura de rede deles e notamos alguns padrões incomuns nos logs do servidor.`)

	byteAnalyzer, err := analyzer.MakeByteAnalyzer(b.Context(), slog.New(slog.NewTextHandler(io.Discard, nil)), analyzer.RunnerOptions{})
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output := &bytes.Buffer{}
		_ = byteAnalyzer.Anonymize(b.Context(), rules, output, input)
	}
}
