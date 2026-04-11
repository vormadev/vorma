package mailutil

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
)

var ErrInvalidEmail = errors.New("invalid email format")

type RawTrimmed string
type Normalized string

type Email struct {
	RawTrimmed
	Normalized
}

func NormalizeEmail(rawEmail string) (*Email, error) {
	rawTrimmed := strings.TrimSpace(rawEmail)
	if rawTrimmed == "" {
		return nil, ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(rawTrimmed)
	if err != nil {
		return nil, ErrInvalidEmail
	}
	atIdx := strings.LastIndex(addr.Address, "@")
	if atIdx < 1 || atIdx == len(addr.Address)-1 {
		return nil, ErrInvalidEmail
	}
	local := strings.ToLower(addr.Address[:atIdx])
	if len(local) > 64 {
		return nil, ErrInvalidEmail
	}
	localCharsetOK := validateLocalCharset(local)
	if !localCharsetOK {
		return nil, ErrInvalidEmail
	}
	if hasLeadingOrTrailingDotOrDoubleDot(local) {
		return nil, ErrInvalidEmail
	}
	domain := addr.Address[atIdx+1:]
	if len(domain) > 255 {
		return nil, ErrInvalidEmail
	}
	domain, err = idna.Lookup.ToASCII(domain)
	if err != nil {
		return nil, ErrInvalidEmail
	}
	domain = strings.ToLower(domain)
	if !strings.Contains(domain, ".") || hasLeadingOrTrailingDotOrDoubleDot(domain) {
		return nil, ErrInvalidEmail
	}
	if domain == "gmail.com" || domain == "googlemail.com" {
		domain = "gmail.com"
		local = strings.ReplaceAll(local, ".", "")
		if idx := strings.Index(local, "+"); idx != -1 {
			local = local[:idx]
		}
	}
	if local == "" {
		return nil, ErrInvalidEmail
	}
	return &Email{
		RawTrimmed: RawTrimmed(rawTrimmed),
		Normalized: Normalized(local + "@" + domain),
	}, nil
}

func validateLocalCharset(input string) bool {
	for _, r := range input {
		if r > unicode.MaxASCII {
			return false
		}
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '+', r == '-':
		default:
			return false
		}
	}
	return true
}

func hasLeadingOrTrailingDotOrDoubleDot(input string) bool {
	if len(input) == 0 {
		return false
	}
	if strings.Contains(input, "..") {
		return true
	}
	return input[0] == '.' || input[len(input)-1] == '.'
}
