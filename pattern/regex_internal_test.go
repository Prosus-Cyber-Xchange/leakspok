package pattern

import (
	"testing"
)

func TestValidatePhoneFormat2Parenthetical(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		// Valid parenthetical format: (+XX) xx xxx xxxx
		{"valid BR phone", "(+55) 11 999 8888", true},
		{"valid intl phone", "(+33) 20 345 6789", true},

		// Invalid: no closing parenthesis
		{"missing closing paren", "(+55 11 999 8888", false},

		// Invalid: wrong country code format
		{"missing plus in cc", "(55) 11 999 8888", false},
		{"too long cc", "(+555) 11 999 8888", false},
		{"too short cc", "(+5) 11 999 8888", false},
		{"non-digit in cc", "(+AB) 11 999 8888", false},

		// Invalid: missing space after closing paren
		{"no space after paren", "(+55)11 999 8888", false},

		// Invalid: wrong number of parts
		{"one part after code", "(+55) 119998888", false},
		{"two parts after code", "(+55) 11 9998888", false},
		{"four parts after code", "(+55) 11 999 8888 123", false},

		// Invalid: wrong digit grouping
		{"wrong first group digits", "(+55) 1 999 8888", false},
		{"wrong second group digits", "(+55) 11 99 8888", false},
		{"wrong third group digits", "(+55) 11 999 888", false},

		// Invalid: non-digit characters
		{"letters in groups", "(+55) AB CDE FGHI", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := validatePhoneFormat2Parenthetical([]byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}

func TestValidatePhoneFormat2Plus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		// Valid plus format: +XX xx xxx xxxx
		{"valid BR phone", "+55 11 999 8888", true},
		{"valid intl phone", "+33 20 345 6789", true},

		// Invalid: wrong number of space-separated parts
		{"three parts", "+55 11 9998888", false},
		{"five parts", "+55 11 999 8888 123", false},
		{"one part", "+55119998888", false},

		// Invalid: wrong first part (country code +XX)
		{"missing plus", "55 11 999 8888", false},
		{"too long cc", "+555 11 999 8888", false},
		{"too short cc", "+5 11 999 8888", false},
		{"non-digit in cc", "+AB 11 999 8888", false},

		// Invalid: wrong digit grouping in remaining parts
		{"wrong second part length", "+55 1 999 8888", false},
		{"wrong third part length", "+55 11 99 8888", false},
		{"wrong fourth part length", "+55 11 999 888", false},

		// Invalid: non-digit characters
		{"letters in groups", "+55 AB CDE FGHI", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := validatePhoneFormat2Plus([]byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}
