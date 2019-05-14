package proxy

import (
	"encoding/base64"
	"strings"

	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/log"
)

// ArmorDomain filters duplicate strings in place and returns a slice with
// only unique strings.
func ArmorDomain(slice []string) []string {
	keys := make(map[string]bool)
	var list []string
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func isWildcard(s string) bool {
	return strings.HasPrefix(s, "*.")
}

func IsSubdomain(domain string, toCheck string) bool {
	if strings.HasSuffix(toCheck, domain) {
		return true
	}
	return false
}

func base64Decode(input string, padding int32) (string, bool) {
	encoding := base64.StdEncoding.WithPadding(padding)
	d, err := encoding.DecodeString(input)
	if err != nil {
		return "", false
	}

	return string(d), true
}

func base64Encode(input string, padding int32) string {
	encoding := base64.StdEncoding.WithPadding(padding)
	e := encoding.EncodeToString([]byte(input))

	return e
}

func getPadding(p string) int32 {

	if len(p) > 1 {
		// Fallback to default padding =
		log.Debug("Invalid Base64 padding value [%s], falling back to %s", tui.Red(p), tui.Green(string(Base64Padding)))
		return Base64Padding
	}

	return []rune(p)[0]
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
