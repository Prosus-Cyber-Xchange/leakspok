package pattern_test

import (
	"context"
	"testing"

	"github.com/ifood/leakspok/pattern"
)

func TestMatchTestCreditCard(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"4242424242424242", true},
		{"4012888888881881", true},
		{"1234567890123456", false},
		{"", false},
	}

	for _, test := range tests {
		got := pattern.MatchTestCreditCard(t.Context(), []byte(test.input))
		if got != test.expect {
			t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
		}
	}
}

func TestMatchCPF(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"11144477735", true},
		{"111444777-35", true},
		{"111.44477735", true},
		{"111.444.77735", true},
		{"111.444.777-35", true},
		{`111.444.777-35"`, true},
		{`111.444.777-35"]}`, true},
		{"11144477734", false},
		{"", false},
	}

	for _, test := range tests {
		got := pattern.MatchCPF(t.Context(), []byte(test.input))
		if got != test.expect {
			t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
		}
	}
}

func TestMatchCNPJ(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"11.444.777/0001-61", true},
		{"14.380.200/0001-21", true},
		{"14.380.2000001-21", true},
		{"143802000001-21", true},
		{"14.380.2000001-21", true},
		{"14380200/000121", true},
		{`14380200/000121"`, true},
		{`14380200/000121"]}`, true},
		{`14380200/000122"`, false},
		{"11.444.777/0001-60", false},
		{"", false},
	}

	for _, test := range tests {
		got := pattern.MatchCNPJ(t.Context(), []byte(test.input))
		if got != test.expect {
			t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
		}
	}
}

func TestMatchIPV4(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"192.168.0.1", true},
		{"10.0.1.9", true},
		{"109.200.3.90", true},
		{"109.200.3.260", false},
		{"", false},
	}

	for _, test := range tests {
		got := pattern.MatchIPV4(t.Context(), []byte(test.input))
		if got != test.expect {
			t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
		}
	}
}

func TestMatchIPV6(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"2001:0db8:85a3::8a2e:0370:7334", true},
		{"2001:0db8:85a3::8a2e:0370:7334", true},
		{`2001:0db8:85a3::8a2e:0370:7334"`, true},
		{`2001:0db8:85a3::8a2e:0370:7334"]}`, true},
		{"2001:::::0370:7334", false},
		{"22::gggg", false},
		{"", false},
	}

	for _, test := range tests {
		got := pattern.MatchIPV6(t.Context(), []byte(test.input))
		if got != test.expect {
			t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
		}
	}
}

func TestMatchCNPJV2(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		// Valid alphanumeric CNPJs with letters
		{"valid CNPJ with letters and dots/slash/dash", "12.ABC.345/01DE-35", true},
		{"valid CNPJ with letters no dots", "12ABC345/01DE-35", true},
		{"valid CNPJ with letters no formatting", "12ABC34501DE35", true},

		// Valid with different formatting
		{"valid CNPJ with trailing quote", `12.ABC.345/01DE-35"`, true},
		{"valid CNPJ with trailing quote and brackets", `12.ABC.345/01DE-35"]}`, true},

		// Invalid - all numeric (old format, should fail as no letters)
		{"invalid numeric only CNPJ 1", "11.444.777/0001-61", false},
		{"invalid numeric only CNPJ 2", "14.380.200/0001-21", false},
		{"invalid numeric only CNPJ no formatting", "14380200000121", false},

		// Invalid - wrong length
		{"invalid CNPJ too short", "AB123456", false},
		{"invalid CNPJ too long", "AB12345600017890", false},
		{"invalid empty CNPJ", "", false},

		// Invalid - contains special characters beyond formatting
		{"invalid CNPJ with @ symbol", "AB@123456000178", false},
		{"invalid CNPJ with # symbol", "AB#123456000178", false},

		// Invalid - lowercase letters (should still work as function converts to uppercase)
		{"valid lowercase letters", "12.abc.345/01de-35", true},
		{"valid mixed case letters", "12.Abc.345/01dE-35", true},

		// Invalid - wrong check digits
		{"invalid check digit 36", "12.ABC.345/01DE-36", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := pattern.MatchCNPJV2(t.Context(), []byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}

func TestMatchEmail(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		{"joaosilva@gmail.com", true},
		{"joaosilva@mail.ru", true},
		{"joaosilva@ifood.com.br", true},
		{"joao.silva@ifood.com.br", true},
		{"joao.silva@@ifood.com.br", false},
		{"joao.silva@1.2.3", false},
		{`joao.silva@ifood.com.br"`, true},
		{`joao.silva@ifood.com.br"]}`, true},
		{"", false},
	}

	for _, test := range tests {
		got := pattern.MatchEmail(t.Context(), []byte(test.input))
		if got != test.expect {
			t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
		}
	}
}

func TestMatchRepeatingNumber(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		// Valid: 5 or more consecutive identical digits
		{"11111", true},
		{"00000", true},
		{"99999", true},
		{"123455555", true},
		{"55555abc", true},
		{"aaaa33333bbbb", true},

		// Invalid: less than 5 consecutive identical digits
		{"1111", false},
		{"12345", false},
		{"121212", false},
		{"11223344", false},

		// Edge cases
		{"", false},
		{"abcdef", false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got := pattern.MatchRepeatingNumber(t.Context(), []byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}

func TestMatchFilename(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		// Valid filenames with known extensions
		{"document.pdf", true},
		{"image.png", true},
		{"data.csv", true},
		{"archive.tar.gz", true},
		{"photo.jpeg", true},
		{"movie.mp4", true},
		{"audio.mp3", true},
		{"archive.zip", true},
		{"index.html", true},
		{"script.js", true},
		{"styles.css", true},

		// Note: xyz is in the valid file extensions list (xtel|xyz|gif)
		// Invalid: unknown extensions
		{"data.unknown", false},
		{"data.fakeext", false},

		// Invalid: filenames without extensions
		{"README", false},
		{"Makefile", false},
		{"dockerfile", false},

		// Edge cases
		{"", false},
		{"noextension", false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got := pattern.MatchFilename(t.Context(), []byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}

func TestMatchPhonesWithExts(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		// Valid: phones with extensions
		{"212-555-1234 ext 42", true},
		{"212-555-1234 ext. 42", true},
		{"212-555-1234 #99", true},
		{"212-555-1234 extension 5", true},
		{"212-555-1234 x123", true},

		// Valid: phones with country code and extension
		{"+1 212-555-1234 x42", true},
		{"+1-212-555-1234 ext 42", true},

		// Valid: phones with area code in parentheses and extension
		{"(212) 555-1234 x123", true},

		// Invalid: phones without extensions (extension part is optional but without it may not match)
		// Note: the regex makes the extension optional via (?), so some formats without ext may match

		// Invalid: garbage
		{"", false},
		{"notaphone", false},
		{"just text with no numbers", false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got := pattern.MatchPhonesWithExts(t.Context(), []byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}

func TestMatchUUID(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		// Valid UUIDs (v4-style format: 8-4-4-4-12 hex digits)
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"6ba7b810-9dad-11d1-80b4-00c04fd430c8", true},
		{"123e4567-e89b-12d3-a456-426614174000", true},
		{"ffffffff-ffff-ffff-ffff-ffffffffffff", true},

		// UUID validation requires mandatory dashes at positions 8, 13, 18, 23
		{"550e8400e29b41d4a716446655440000", false},

		// Invalid UUIDs
		{"not-a-uuid", false},
		{"550e8400-e29b-41d4-a716-44665544000", false},   // too short (35 chars)
		{"550e8400-e29b-41d4-a716-4466554400000", false}, // too long (37 chars)
		{"gggggggg-gggg-gggg-gggg-gggggggggggg", false},  // non-hex characters

		// Edge cases
		{"", false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got := pattern.MatchUUID(t.Context(), []byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}

func TestMatchVisaCreditCard(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		// Valid Visa credit card numbers (start with 4, 16 digits)
		{"4242424242424242", true},
		{"4012888888881881", true},
		{"4000056655665556", true},
		{"4111111111111111", true},

		// Valid with formatting (spaces and dashes are optional)
		{"4242 4242 4242 4242", true},
		{"4242-4242-4242-4242", true},

		// Invalid: MasterCard numbers (start with 5)
		{"5555555555554444", false},
		{"5105105105105100", false},

		// Invalid: wrong prefix or length
		{"1234567890123456", false},
		{"424242424242424", false},   // too short (15 digits)
		{"42424242424242424", false}, // too long (17 digits)

		// Edge cases
		{"", false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got := pattern.MatchVisaCreditCard(t.Context(), []byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}

func TestMatchMasterCardCreditCard(t *testing.T) {
	tests := []struct {
		input  string
		expect bool
	}{
		// Valid MasterCard credit card numbers (start with 51-55, 16 digits)
		{"5555555555554444", true},
		{"5105105105105100", true},
		{"5200828282828210", true},
		{"5300000000000000", true},
		{"5454545454545454", true},

		// Valid with formatting (spaces and dashes are optional)
		{"5555 5555 5555 4444", true},
		{"5555-5555-5555-4444", true},

		// Invalid: Visa numbers (start with 4)
		{"4242424242424242", false},
		{"4012888888881881", false},

		// Invalid: wrong prefix or length
		{"1234567890123456", false},
		{"5656565656565656", false},  // 56 is not in [51-55] range
		{"5050505050505050", false},  // 50 is not in [51-55] range
		{"555555555555444", false},   // too short (15 digits)
		{"55555555555544444", false}, // too long (17 digits)

		// Edge cases
		{"", false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got := pattern.MatchMasterCardCreditCard(t.Context(), []byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}

func TestMatchFunctionsEdgeCases(t *testing.T) {
	ctx := context.Background()

	// Generate very long payload
	veryLong := make([]byte, 10240)
	for i := range veryLong {
		veryLong[i] = 'a'
	}

	// All match functions to test for edge cases
	type funcCase struct {
		name string
		fn   func(context.Context, []byte) bool
	}
	funcs := []funcCase{
		{"MatchEmail", pattern.MatchEmail},
		{"MatchCPF", pattern.MatchCPF},
		{"MatchCNPJ", pattern.MatchCNPJ},
		{"MatchCNPJV2", pattern.MatchCNPJV2},
		{"MatchIPV4", pattern.MatchIPV4},
		{"MatchIPV6", pattern.MatchIPV6},
		{"MatchVisaCreditCard", pattern.MatchVisaCreditCard},
		{"MatchMasterCardCreditCard", pattern.MatchMasterCardCreditCard},
		{"MatchVIN", pattern.MatchVIN},
		{"MatchRepeatingNumber", pattern.MatchRepeatingNumber},
		{"MatchFilename", pattern.MatchFilename},
		{"MatchPhonesWithExts", pattern.MatchPhonesWithExts},
		{"MatchUUID", pattern.MatchUUID},
		{"MatchTestCreditCard", pattern.MatchTestCreditCard},
	}

	for _, fc := range funcs {
		t.Run(fc.name+"/empty", func(t *testing.T) {
			if fc.fn(ctx, []byte{}) {
				t.Errorf("%s should not match empty input", fc.name)
			}
		})

		t.Run(fc.name+"/nil", func(t *testing.T) {
			if fc.fn(ctx, nil) {
				t.Errorf("%s should not match nil input", fc.name)
			}
		})

		t.Run(fc.name+"/very_long", func(t *testing.T) {
			if fc.fn(ctx, veryLong) {
				t.Errorf("%s should not match very long random input", fc.name)
			}
		})
	}

	// Unicode test cases specific to each function
	unicodeCases := []struct {
		name   string
		fn     func(context.Context, []byte) bool
		input  []byte
		expect bool
	}{
		{"MatchEmail unicode", pattern.MatchEmail, []byte(""), false},
		{"MatchCPF unicode", pattern.MatchCPF, []byte("café☕"), false},
		{"MatchCNPJ unicode", pattern.MatchCNPJ, []byte("naïve"), false},
		{"MatchCNPJV2 unicode", pattern.MatchCNPJV2, []byte("日本語"), false},
		{"MatchIPV4 unicode", pattern.MatchIPV4, []byte("rêsumé"), false},
		{"MatchIPV6 unicode", pattern.MatchIPV6, []byte("über"), false},
		{"MatchVisaCreditCard unicode", pattern.MatchVisaCreditCard, []byte("caffè"), false},
		{"MatchMasterCardCreditCard unicode", pattern.MatchMasterCardCreditCard, []byte("ñandú"), false},
		{"MatchVIN unicode", pattern.MatchVIN, []byte("café"), false},
		{"MatchRepeatingNumber unicode", pattern.MatchRepeatingNumber, []byte("Über"), false},
		{"MatchFilename unicode", pattern.MatchFilename, []byte("中文.pdf"), false},
		{"MatchPhonesWithExts unicode", pattern.MatchPhonesWithExts, []byte("café"), false},
		{"MatchUUID unicode", pattern.MatchUUID, []byte("café"), false},
		{"MatchTestCreditCard unicode", pattern.MatchTestCreditCard, []byte("Über"), false},
	}

	for _, uc := range unicodeCases {
		t.Run(uc.name, func(t *testing.T) {
			got := uc.fn(ctx, uc.input)
			if got != uc.expect {
				t.Errorf("For input %q expected %v but got %v", uc.input, uc.expect, got)
			}
		})
	}
}

func TestMatchPatternsInFilePaths(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		matcher func(context.Context, []byte) bool
		expect  bool
	}{
		// Email in file paths with multiple forward slashes
		{"email in file path", "/home/user/john.silva@example.com/documents/file.txt", pattern.MatchEmail, true},
		{"email in deep file path", "/data/users/joao.silva@ifood.com.br/backups/2024/file.zip", pattern.MatchEmail, true},
		{"email with multiple slashes", "/home//user//joaosilva@gmail.com//docs/", pattern.MatchEmail, true},
		{"email in path with trailing slash", "/var/logs/user@example.com/", pattern.MatchEmail, true},

		// CPF in file paths with multiple forward slashes
		{"CPF in file path", "/data/users/11144477735/personal/file.pdf", pattern.MatchCPF, true},
		{"CPF formatted in file path", "/backup/111.444.777-35/documents/archive.tar.gz", pattern.MatchCPF, true},
		{"CPF with multiple path levels", "/home/user/11144477735/documents/", pattern.MatchCPF, true},
		{"CPF in path with trailing slash", "/data/11144477735/", pattern.MatchCPF, true},
		{"CPF partially formatted in path", "/users/111.444.77735/reports/data.csv", pattern.MatchCPF, true},
		//todo: think about if we want to support this case
		// {"CPF with multiple path levels", "/archive/users/11144477735/2024/backup/backup.zip", pattern.MatchCPF, true},

		// Phone in file paths with multiple forward slashes
		{"phone in file path", "/contacts/11999999999/directory/info.txt", pattern.MatchPhone, true},
		{"phone with multiple path levels", "/phones/archive/11987654321/2024/contacts.json", pattern.MatchPhone, true},
		{"phone in path with trailing slash", "/mobile/11998765432/", pattern.MatchPhone, true},

		// Non-matching patterns in file paths
		{"invalid CPF in file path", "/data/11144477734/backup/file.txt", pattern.MatchCPF, false},
		{"no email in numeric path", "/data/12345678900/docs/file.txt", pattern.MatchEmail, false},
		{"empty string", "", pattern.MatchEmail, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.matcher(t.Context(), []byte(test.input))
			if got != test.expect {
				t.Errorf("For input %q expected %v but got %v", test.input, test.expect, got)
			}
		})
	}
}
