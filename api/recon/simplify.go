package recon

import (
	"fmt"
	"strings"

	"github.com/icza/abcsort"

	"github.com/muraenateam/muraena/core/proxy"
)

func reverseString(ss []string) []string {
	last := len(ss) - 1
	for i := 0; i < len(ss)/2; i++ {
		ss[i], ss[last-i] = ss[last-i], ss[i]
	}
	return ss
}

// SimplifyDomains collapses 3rd/4th-level subdomains to wildcards and sorts them.
// Relocated from the (removed) module/crawler package.
func SimplifyDomains(input []string) []string {
	var domains []string
	for _, d := range input {
		host := strings.TrimSpace(d)
		parts := reverseString(strings.Split(host, "."))
		switch len(parts) {
		case 3:
			host = fmt.Sprintf("*.%s.%s", parts[1], parts[0])
		case 4:
			host = fmt.Sprintf("*.%s.%s.%s", parts[2], parts[1], parts[0])
		}
		domains = append(domains, host)
	}
	domains = proxy.ArmorDomain(domains)
	abcsort.New("*").Strings(domains)
	return domains
}
