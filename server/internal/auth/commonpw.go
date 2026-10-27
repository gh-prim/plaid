package auth

import (
	_ "embed"
	"strings"
)

//go:embed common_passwords.txt
var commonPasswordsFile string

var commonPasswords = func() map[string]struct{} {
	set := map[string]struct{}{}
	for _, line := range strings.Split(commonPasswordsFile, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			set[strings.ToLower(line)] = struct{}{}
		}
	}
	return set
}()

func isCommonPassword(pw string) bool {
	_, ok := commonPasswords[strings.ToLower(pw)]
	return ok
}
