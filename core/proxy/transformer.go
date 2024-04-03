package proxy

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/evilsocket/islazy/tui"
	"golang.org/x/net/html"

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
// TODO: the b64 can be set into the Replacer struct
func (r *Replacer) Transform(input string, forward bool, b64 Base64, repetitions ...int) (result string) {

	// Panic recovery
	defer func() {
		if err := recover(); err != nil {
			log.Warning("Recovered from panic: %s", err)
		}
	}()

	// source := strings.TrimSpace(input)
	source := input
	if strings.TrimSpace(input) == "" {
		return source
	}

	count := 0
	if len(repetitions) > 0 {
		count = repetitions[0]
	}

	var replacements []string
	var lastReplacements []string
	if forward { // used in Requests
		// if source contains ---XXwld, we need to patch the wildcard
		wildCardSeparator := r.getCustomWildCardSeparator()
		if strings.Contains(source, wildCardSeparator) {
			// Extract all urls from the source containing ---XXwld and patch the wildcard
			re := regexp.MustCompile(fmt.Sprintf(`%s\d+.%s`, r.WildcardRegex(true), r.Phishing))
			matchSubdomains := re.FindAllString(source, -1)
			matchSubdomains = ArmorDomain(matchSubdomains)
			if len(matchSubdomains) > 0 {
				log.Verbose("Wildcard pattern: %v match %d", re.String(), len(matchSubdomains))

				for _, element := range matchSubdomains {
					newDomain := r.PatchComposedWildcardURL(element)
					source = strings.ReplaceAll(source, element, newDomain)
				}

				log.Verbose("Source after wildcard patching: %s", source)
			}
		}

		replacements = r.GetForwardReplacements()
		lastReplacements = r.GetLastForwardReplacements()
	} else {
		// used in Responses
		replacements = r.GetBackwardReplacements()
		lastReplacements = r.GetLastBackwardReplacements()
	}

	// Handling of base64 encoded data which should be decoded before transformation
	source, base64Found, padding := transformBase64(source, b64, true, Base64Padding)

	if count > 2 {
		log.Verbose("Too many transformation loops, switch to a case insentive replace:")
		// Replace transformation
		var err error
		result, err = caseInsensitiveReplace(source, replacements)
		if err != nil {
			log.Error(err.Error())
			return
		}

		// do last replacements
		result, err = caseInsensitiveReplace(result, lastReplacements)
		if err != nil {
			log.Error(err.Error())
			return
		}

	} else {
		// Replace transformation
		result = strings.NewReplacer(replacements...).Replace(source)
		// do last replacements
		result = strings.NewReplacer(lastReplacements...).Replace(result)
	}

	// Re-encode if base64 encoded data was found
	if base64Found {
		result, _, _ = transformBase64(result, b64, false, padding)
	}

	// TODO this could go into trasform URL with the forward parameter..
	if !forward {
		// if it's a URL, we need to transform the query string as well
		// if begins with http:// or https://
		if strings.HasPrefix(result, "http://") || strings.HasPrefix(result, "https://") {
			newURL, err := r.transformBackwardUrl(result, b64)
			if err == nil {
				result = newURL
			}
		}

		if strings.HasPrefix(result, "<html") || strings.HasPrefix(result, "<!DOCTYPE") {
			if strings.Contains(result, "://") {
				newResult := result
				urls := extractURLsFromHTML(result)
				for _, url := range urls {
					newURL, err := r.transformBackwardUrl(url, b64)
					if err == nil {
						newResult = strings.ReplaceAll(newResult, url, newURL)
					}
				}

				result = newResult
			}
		}
	}

	// If the result is the same as the input, we don't need to do anything else
	if source != result {
		// Find wildcard matching
		if Wildcards {

			wldPrefix := r.WildcardPrefix()

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
				re := regexp.MustCompile(fmt.Sprintf(`%s\d+.%s`, r.WildcardRegex(false), domain))
				matchSubdomains := re.FindAllString(result, -1)
				matchSubdomains = ArmorDomain(matchSubdomains)
				if len(matchSubdomains) > 0 {
					log.Verbose("Wildcard pattern: %v match %d!", re.String(), len(matchSubdomains))
				}

				for _, element := range matchSubdomains {
					if core.StringContains(element, rep) {
						continue
					}

					if strings.HasPrefix(element, r.getCustomWildCardSeparator()) {
						continue
					}

					if strings.HasPrefix(element, "."+wldPrefix) {
						continue
					}

					// Patch the wildcard
					element = strings.ReplaceAll(element, "."+wldPrefix, CustomWildcardSeparator+wldPrefix)
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
					count++
					log.Verbose("We need another (#%d) transformation loop, because of this new domains:%s",
						count, tui.Green(fmt.Sprintf("%v", rep)))

					if count > 5 {
						log.Debug("Too many transformation loops, aborting.")
						return
					}

					// Pass count in as a parameter to avoid infinite loop
					return r.Transform(input, forward, b64, count)
				}
			}
		}
	}

	return
}

func extractURLsFromHTML(htmlContent string) []string {
	var urls []string
	tokenizer := html.NewTokenizer(strings.NewReader(htmlContent))

	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			return urls
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						// Manually re-encode the URL to preserve the original encoding
						encodedURL := html.EscapeString(attr.Val)
						urls = append(urls, encodedURL)
					}
				}
			}
		}
	}
}

func (r *Replacer) PatchComposedWildcardURL(URL string) (result string) {
	result = URL

	wldPrefix := fmt.Sprintf("%s%s", r.ExternalOriginPrefix, WildcardLabel)
	if strings.Contains(result, CustomWildcardSeparator+wldPrefix) {

		// URL decode result
		decodedValue, err := url.QueryUnescape(result)
		if err == nil && result != decodedValue {
			result = decodedValue
		}

		// this could be a nested url, so we need to extract the subdomain
		// we need to remove the protocol, anything before ://
		if strings.Contains(result, "://") {
			// starting from last occurrence of ://
			c := strings.Split(result, "://")
			result = c[len(c)-1]
		}

		subdomain := strings.Split(result, CustomWildcardSeparator)[0]
		wildcard := strings.Split(result, CustomWildcardSeparator)[1]
		// remove r.Phishing from the wildcard
		wildcard = strings.Split(wildcard, "."+r.Phishing)[0]
		wildcard = strings.Split(wildcard, "/")[0] // just in case...

		path := ""
		if strings.Contains(result, r.Phishing+"/") {
			path = "/" + strings.Split(result, r.Phishing+"/")[1]
		}
		// wildcard = strings.TrimSuffix(wildcard, fmt.Sprintf(".%s", r.Phishing))

		domain := fmt.Sprintf("%s.%s", subdomain, r.patchWildcard(wildcard))
		log.Info("Wildcard to patch: %s (%s)", tui.Bold(tui.Red(result)), tui.Green(domain))

		// remove the protocol, anything befgore ://
		protocol := ""
		if strings.Contains(domain, "://") {
			protocol = strings.Split(domain, "://")[0] + "://"
			domain = strings.Split(domain, "://")[1]
		}
		r.SetExternalOrigins([]string{domain})
		if err := r.DomainMapping(); err != nil {
			log.Error(err.Error())
			return
		}

		// origins := r.GetOrigins()
		// if sub, ok := origins[domain]; ok {
		//	log.Info("%s is mapped to %s", tui.Bold(tui.Red(domain)), tui.Green(sub))
		// }

		if err := r.Save(); err != nil {
			log.Error("Error saving replacer: %s", err)
		}

		r.MakeReplacements()

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
			// encode
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

// TODO merge the transformURL and transformBackwardURL
func (r *Replacer) transformBackwardUrl(URL string, base64 Base64) (result string, err error) {
	result = URL

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

	queryParts := strings.Split(hURL.RawQuery, "&")

	// Create a slice to store key-value pairs
	var params []string

	for _, part := range queryParts {
		keyValue := strings.Split(part, "=")
		if len(keyValue) == 2 {
			params = append(params, part)
		}
	}

	// Modify the values
	for i, param := range params {
		keyValue := strings.Split(param, "=")
		if len(keyValue) == 2 {
			key := keyValue[0]
			value := keyValue[1]

			// URL-decode the value
			decodedValue, err := url.QueryUnescape(value)
			if err == nil {
				value = r.Transform(decodedValue, false, base64)
			} else {
				value = r.Transform(value, false, base64)
			}

			// URL-encode the modified value
			encodedValue := url.QueryEscape(value)

			params[i] = key + "=" + encodedValue

		}
	}

	// Join the modified key-value pairs and set them as the new RawQuery
	hURL.RawQuery = strings.Join(params, "&")
	result = hURL.String()
	return
}

// PatchWildcardList patches the wildcard domains in the list
// and returns the patched list of domains to be used in the replacer
// TODO: RENAME ME
func (r *Replacer) patchWildcardList(rep []string) (prep []string) {
	rep = ArmorDomain(rep)
	for _, s := range rep {

		x := strings.Split(s, CustomWildcardSeparator)
		if len(x) < 2 {
			continue
		}

		subdomain := strings.Split(s, CustomWildcardSeparator)[0]
		wildcard := strings.Split(s, CustomWildcardSeparator)[1]
		wildcard = strings.Split(wildcard, "/")[0]

		// domain := r.patchWildcard(s)
		domain := fmt.Sprintf("%s.%s", subdomain, r.patchWildcard(wildcard))
		if domain != "" {
			prep = append(prep, domain)
		}
	}

	return prep
}

func (r *Replacer) patchWildcard(rep string) (prep string) {
	found := false
	newDomain := strings.TrimSuffix(rep, fmt.Sprintf(".%s", r.Phishing))

	for w, d := range r.GetWildcardMapping() {
		if strings.HasSuffix(newDomain, d) {

			// TODO: This is unclear ..
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

	r.SetForwardReplacements([]string{})
	r.SetForwardReplacements(append(r.ForwardReplacements, []string{r.Phishing, r.Target}...))

	// Add the SubdomainMap to the forward replacements
	for _, sub := range r.SubdomainMap {
		from := fmt.Sprintf("%s.%s", sub, r.Phishing)
		to := fmt.Sprintf("%s.%s", sub, r.Target)
		rep := []string{from, to}
		r.SetForwardReplacements(append(r.ForwardReplacements, rep...))
	}

	log.Verbose("[Forward | Origins]: %d", len(origins))
	count := len(r.ForwardReplacements)
	for extOrigin, subMapping := range origins { // changes resource-1.phishing.

		if strings.HasPrefix(subMapping, WildcardLabel) {
			// Ignoring wildcard at this stage
			log.Debug("[Wildcard] %s - %s", tui.Yellow(subMapping), tui.Green(extOrigin))
			continue
		}

		from := fmt.Sprintf("%s.%s", subMapping, r.Phishing)
		to := extOrigin
		rep := []string{from, to}
		r.SetForwardReplacements(append(r.ForwardReplacements, rep...))

		count++
		log.Verbose("[Forward | replacements #%d]: %s > %s", count, tui.Yellow(rep[0]), tui.Green(to))
	}

	// Append wildcards at the end
	r.SetForwardWildcardReplacements([]string{})
	for extOrigin, subMapping := range r.GetWildcardMapping() {
		from := fmt.Sprintf("%s.%s", subMapping, r.Phishing)
		to := extOrigin
		rep := []string{from, to}
		r.SetForwardWildcardReplacements(append(r.ForwardWildcardReplacements, rep...))

		count++
		log.Verbose("[Wild Forward | replacements #%d]: %s > %s", count, tui.Yellow(rep[0]), tui.Green(to))
	}

	//
	// Responses
	//
	r.SetBackwardReplacements([]string{})

	//
	// Dirty fix for the case when the Victim domain is a subdomain of the Phishing domain:
	// e.g. phishing.com and no-phishing.com
	//
	// Define potential boundaries around the domain
	boundaries := []string{" ", ",", ".", ":", "/", "(", ")", "!", "'", "\"", ";", "<", ">", "\n", "\t"}
	targetVariations, phishingVariations := createVariations(r.Target, r.Phishing, boundaries)
	for i, variation := range targetVariations {
		r.SetBackwardReplacements(append(r.BackwardReplacements, []string{variation, phishingVariations[i]}...))
	}

	// Add the SubdomainMap to the backward replacements
	for _, sub := range r.SubdomainMap {
		from := fmt.Sprintf("%s.%s", sub, r.Target)
		to := fmt.Sprintf("%s.%s", sub, r.Phishing)
		rep := []string{from, to}
		r.SetBackwardReplacements(append(r.BackwardReplacements, rep...))
	}

	count = 0
	for include, subMapping := range origins {

		if strings.HasPrefix(subMapping, WildcardLabel) {
			// Ignoring wildcard at this stage
			log.Verbose("[Wildcard] %s - %s", tui.Yellow(subMapping), tui.Green(include))
			continue
		}

		from := include
		to := fmt.Sprintf("%s.%s", subMapping, r.Phishing)
		rep := []string{from, to}
		r.SetBackwardReplacements(append(r.BackwardReplacements, rep...))

		count++
		log.Verbose("[Backward | replacements #%d]: %s < %s", count, tui.Green(rep[0]), tui.Yellow(to))
	}

	// Append wildcards at the end
	r.SetBackwardWildcardReplacements([]string{})
	for include, subMapping := range r.GetWildcardMapping() {
		from := include
		to := fmt.Sprintf("%s.%s", subMapping, r.Phishing)
		rep := []string{from, to}
		r.SetBackwardWildcardReplacements(append(r.BackwardWildcardReplacements, rep...))
		count++
		log.Verbose("[Wild Backward | replacements #%d]: %s < %s", count, tui.Green(rep[0]), tui.Yellow(to))
	}

	// These should be done as Final replacements
	r.SetLastBackwardReplacements([]string{})

	// CustomContent HTTP response replacements
	for _, tr := range r.CustomResponseTransformations {
		r.SetLastBackwardReplacements(append(r.LastBackwardReplacements, tr...))
		log.Verbose("[CustomContent Replacements] %+v", tr)
	}

	r.SetLastBackwardReplacements(append(r.BackwardReplacements, r.LastBackwardReplacements...))

}

// createVariations generates all possible variations of the target and phishing strings with boundaries
func createVariations(target, phishing string, boundaries []string) ([]string, []string) {
	var targetVariations, phishingVariations []string

	// Generate variations with each boundary preceding and following the target
	for _, boundary := range boundaries {
		targetVariations = append(targetVariations, boundary+target)
		phishingVariations = append(phishingVariations, boundary+phishing)
	}

	return targetVariations, phishingVariations
}

func (r *Replacer) DomainMapping() (err error) {
	baseDom := r.Target
	// log.Debug("Proxy destination: %s", tui.Bold(tui.Green("*."+baseDom)))

	origins := make(map[string]string)
	r.WildcardMapping = make(map[string]string)

	count, wildcards := 0, 0
	for _, domain := range r.GetExternalOrigins() {
		if IsSubdomain(baseDom, domain) {
			trim := strings.TrimSuffix(domain, baseDom)

			// We don't map 1-level subdomains ..
			if strings.Count(trim, ".") < 2 {
				log.Verbose("Ignore: %s [%s]", domain, trim)
				continue
			}
		}

		if isWildcard(domain) {
			// Wildcard domains
			wildcards++

			// Fix domain by removing the prefix *.
			domain = strings.TrimPrefix(domain, "*.")

			// Update the wildcard map
			prefix := fmt.Sprintf("%s%s", r.ExternalOriginPrefix, WildcardLabel)
			o := fmt.Sprintf("%s%d", prefix, wildcards)
			r.SetWildcardDomain(o)
			r.SetWildcardMapping(domain, o)
			// log.Debug(fmt.Sprintf("*.%s=%s", domain, o))

		} else {
			count++
			// Extra domains or nested subdomains
			o := fmt.Sprintf("%s%d", r.ExternalOriginPrefix, count)
			origins[domain] = o
			// log.Debug(fmt.Sprintf("%s=%s", domain, o))
		}

	}

	if wildcards > 0 {
		Wildcards = true
	}

	r.SetOrigins(origins)
	// log.Verbose("Processed %d domains to transform, %d are wildcards", count, wildcards)
	return
}
