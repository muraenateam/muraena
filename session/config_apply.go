package session

import (
	"encoding/json"
	"strings"
)

var restartPrefixes = []string{
	"Proxy.Port", "Proxy.IP", "Proxy.Listener", "Proxy.HTTPtoHTTPS",
	"TLS", "Api.Bind", "Api.Port",
}

// CollectPaths returns dotted paths to every leaf in a nested patch map.
func CollectPaths(patch map[string]interface{}) []string {
	var out []string
	var walk func(prefix string, m map[string]interface{})
	walk = func(prefix string, m map[string]interface{}) {
		for k, v := range m {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			if child, ok := v.(map[string]interface{}); ok && len(child) > 0 {
				walk(path, child)
			} else {
				out = append(out, path)
			}
		}
	}
	walk("", patch)
	return out
}

// RestartPrefixes returns a copy of the static list of config path prefixes
// that require a process restart to take effect.
func RestartPrefixes() []string { return append([]string(nil), restartPrefixes...) }

// RestartRequiredFields returns the paths that require a restart.
func RestartRequiredFields(paths []string) []string {
	var rr []string
	for _, p := range paths {
		for _, pre := range restartPrefixes {
			if p == pre || strings.HasPrefix(p, pre+".") {
				rr = append(rr, p)
				break
			}
		}
	}
	return rr
}

// DeepMerge recursively merges src into dst and returns dst.
func DeepMerge(dst, src map[string]interface{}) map[string]interface{} {
	for k, sv := range src {
		if sm, ok := sv.(map[string]interface{}); ok {
			if dm, ok := dst[k].(map[string]interface{}); ok {
				dst[k] = DeepMerge(dm, sm)
				continue
			}
		}
		dst[k] = sv
	}
	return dst
}

// BuildCandidate clones the current config, merges the patch in JSON space, and
// returns the candidate plus any restart-required paths the patch touched.
func (s *Session) BuildCandidate(patch map[string]interface{}) (*Configuration, []string, error) {
	current := s.Config()
	b, err := json.Marshal(current)
	if err != nil {
		return nil, nil, err
	}
	var curMap map[string]interface{}
	if err := json.Unmarshal(b, &curMap); err != nil {
		return nil, nil, err
	}
	merged := DeepMerge(curMap, patch)

	mb, err := json.Marshal(merged)
	if err != nil {
		return nil, nil, err
	}
	var cand Configuration
	if err := json.Unmarshal(mb, &cand); err != nil {
		return nil, nil, err
	}

	restart := RestartRequiredFields(CollectPaths(patch))
	return &cand, restart, nil
}
