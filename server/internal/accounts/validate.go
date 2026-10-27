package accounts

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gh-prim/plaid/server/internal/apierr"
)

var (
	usernameRe = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)
	// localeRe accepts BCP 47 language tags as used by the clients: "fr",
	// "pt-BR", "zh-Hant", "es-419".
	localeRe = regexp.MustCompile(`^[a-z]{2,3}(-([A-Z]{2}|[0-9]{3}|[A-Z][a-z]{3}))?$`)
	pinRe    = regexp.MustCompile(`^[0-9]{4}$`)
)

// allowedMaxAges are the age limits a profile can have (F003-R18).
var allowedMaxAges = []int{0, 7, 10, 12, 13, 16, 18}

// deviceTypes mirrors the sessions.device_type CHECK constraint.
var deviceTypes = []string{"web", "ios", "ipados", "tvos", "android", "androidtv", "other"}

const maxProfiles = 8

// problems collects invalid fields for one validation_failed error.
type problems []apierr.Detail

func (p *problems) add(field, code string) { *p = append(*p, apierr.Detail{Field: field, Code: code}) }

func (p problems) err() error {
	if len(p) == 0 {
		return nil
	}
	return apierr.ValidationFailed(p...)
}

// normalizeUsername lowercases and checks a username (F003-R4).
func normalizeUsername(raw string, p *problems) string {
	u := strings.ToLower(strings.TrimSpace(raw))
	if !usernameRe.MatchString(u) {
		p.add("username", "invalid")
	}
	return u
}

// checkLength trims value and checks it has 1 to maxLen characters.
func checkLength(field, value string, maxLen int, p *problems) string {
	v := strings.TrimSpace(value)
	switch n := utf8.RuneCountInString(v); {
	case n == 0:
		p.add(field, "too_short")
	case n > maxLen:
		p.add(field, "too_long")
	}
	return v
}

func checkDisplayName(v string, p *problems) string { return checkLength("displayName", v, 64, p) }

func checkLocale(v string, p *problems) string {
	if !localeRe.MatchString(v) {
		p.add("locale", "invalid")
	}
	return v
}

func checkDevice(d Device, p *problems) Device {
	d.Name = checkLength("device.name", d.Name, 64, p)
	if !slices.Contains(deviceTypes, d.Type) {
		p.add("device.type", "invalid")
	}
	if utf8.RuneCountInString(d.ClientVersion) > 32 {
		p.add("device.clientVersion", "too_long")
	}
	return d
}

func checkPIN(field, pin string, p *problems) {
	if !pinRe.MatchString(pin) {
		p.add(field, "invalid")
	}
}

func checkMaxAge(age int, p *problems) {
	if !slices.Contains(allowedMaxAges, age) {
		p.add("maxAge", "invalid")
	}
}

func checkAvatarColor(c int, p *problems) {
	if c < 0 || c > 7 {
		p.add("avatarColor", "invalid")
	}
}
