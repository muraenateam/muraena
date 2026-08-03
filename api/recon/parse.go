package recon

import (
	"encoding/json"

	"github.com/muraenateam/muraena/session"
)

type LoginPage struct {
	URL              string `json:"url"`
	Action           string `json:"action"`
	UsernameSelector string `json:"usernameSelector"`
	PasswordSelector string `json:"passwordSelector"`
}

type Pattern struct {
	Label    string `json:"label"`
	Matching string `json:"matching"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

type Result struct {
	Target          string      `json:"target"`
	Origins         []string    `json:"origins"`
	LoginPages      []LoginPage `json:"loginPages"`
	SecretsPaths    []string    `json:"secretsPaths"`
	SecretsPatterns []Pattern   `json:"secretsPatterns"`
}

func Parse(stdout []byte) (*Result, error) {
	var r Result
	if err := json.Unmarshal(stdout, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ToPatch builds a config patch (Go-field-name space) merging discovered origins
// and secrets into the current config.
func (r *Result) ToPatch(current *session.Configuration) map[string]interface{} {
	merged := SimplifyDomains(dedupe(append(append([]string{}, current.Origins.ExternalOrigins...), r.Origins...)))

	paths := dedupe(append(append([]string{}, current.Tracking.Secrets.Paths...), r.SecretsPaths...))

	// patterns: represent as []map to match JSON field names on Configuration
	var patterns []map[string]string
	for _, p := range current.Tracking.Secrets.Patterns {
		patterns = append(patterns, map[string]string{
			"Label": p.Label, "Matching": p.Matching, "Start": p.Start, "End": p.End,
		})
	}
	for _, p := range r.SecretsPatterns {
		patterns = append(patterns, map[string]string{
			"Label": p.Label, "Matching": p.Matching, "Start": p.Start, "End": p.End,
		})
	}

	return map[string]interface{}{
		"Origins": map[string]interface{}{
			"ExternalOrigins": merged,
		},
		"Tracking": map[string]interface{}{
			"Secrets": map[string]interface{}{
				"Paths":    paths,
				"Patterns": patterns,
			},
		},
	}
}
