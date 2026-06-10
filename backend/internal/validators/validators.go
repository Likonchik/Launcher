package validators

import (
	"regexp"
	"strings"
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

func IsValidUsername(s string) bool {
	return usernameRe.MatchString(strings.TrimSpace(s))
}

// IsValidNickname — то же правило, что для логина/игрового ника в боте.
func IsValidNickname(s string) bool {
	return IsValidUsername(s)
}

func IsValidEmail(loose bool, s string) bool {
	t := strings.TrimSpace(s)
	if len(t) < 5 || len(t) > 254 {
		return false
	}
	if loose {
		return strings.Contains(t, "@") && !strings.HasPrefix(t, "@") && !strings.HasSuffix(t, "@")
	}
	return strings.Contains(t, "@") && !strings.HasPrefix(t, "@") && !strings.HasSuffix(t, "@") &&
		!strings.Contains(t, " ")
}

func IsValidPassword(pw string) bool {
	n := len(pw)
	return n >= 8 && n <= 128
}
