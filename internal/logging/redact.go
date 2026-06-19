package logging

import (
	"net/url"
	"strings"
)

// sensitiveKeys are URL query parameter substrings whose values are redacted.
var sensitiveKeys = []string{"token", "key", "secret", "password", "auth", "api_key", "apikey", "access_token"}

// RedactString redacts credentials in a URL string: userinfo is replaced with
// [REDACTED] and any query parameter whose name contains a sensitive key is
// redacted. Non-URL strings are returned unchanged.
func RedactString(s string) string {
	if s == "" {
		return s
	}
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" {
		if u.User != nil {
			u.User = url.User("[REDACTED]")
		}
		q := u.Query()
		for k := range q {
			lk := strings.ToLower(k)
			for _, needle := range sensitiveKeys {
				if strings.Contains(lk, needle) {
					q.Set(k, "[REDACTED]")
				}
			}
		}
		u.RawQuery = q.Encode()
		return u.String()
	}
	return s
}
