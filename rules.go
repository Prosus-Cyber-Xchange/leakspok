package leakspok

import (
	"github.com/Prosus-Cyber-Xchange/leakspok/analyzer"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
)

// DefaultMaskString is used to mask matches. It's useful when should report leaks on security alerts
const DefaultMaskString = "*"

// DefaultRedactString is used to redact matches. It's useful when should report leaks on security alerts
const DefaultRedactString = "<REDACTED>"

//nolint:gochecknoglobals // default rules should be global
var (
	DefaultRuleSetting = analyzer.RuleSettings{
		Strategy: analyzer.REDACT,
		Redact: &analyzer.RedactSettings{
			Placeholder: DefaultRedactString,
		},
		Mask: &analyzer.MaskSettings{
			MaskingChar: DefaultMaskString,
		},
	}

	// DefaultCPFRule is a default rule for Brazilian CPF
	DefaultCPFRule = analyzer.Rule{
		Name:        "brazilian_cpf",
		Description: "Brazilian CPF",
		Matcher:     pattern.CPFMatcher(),
		Settings:    DefaultRuleSetting,
	}

	// DefaultCNPJRule is a default rule for Brazilian CNPJ
	DefaultCNPJRule = analyzer.Rule{
		Name:        "brazilian_cnpj",
		Description: "Brazilian CNPJ",
		Matcher:     pattern.CNPJMatcher(),
		Settings:    DefaultRuleSetting,
	}

	// DefaultEmailRule is a default rule for email address
	DefaultEmailRule = analyzer.Rule{
		Name:        "email_address",
		Description: "valid email address",
		Matcher:     pattern.EmailMatcher(),
		Settings:    DefaultRuleSetting,
	}

	// DefaultIPRule is a default rule for IP address
	DefaultIPRule = analyzer.Rule{
		Name:        "ip_address",
		Description: "valid IPv4 or IPv6 address",
		Matcher:     pattern.IPMatcher(),
		Settings:    DefaultRuleSetting,
	}

	// DefaultCreditCardRule is a default rule for credit card number
	DefaultCreditCardRule = analyzer.Rule{
		Name:        "credit_card",
		Description: "valid credit card number",
		Matcher:     pattern.CreditCardMatcher(),
		Settings:    DefaultRuleSetting,
	}
)
