package proxy

import (
	"encoding/json"
	"io/ioutil"
	"sync"
)

const ReplaceFile = "session.json"

// Replacer structure used to populate the transformation rules
type Replacer struct {
	Phishing                      string
	Target                        string
	ExternalOrigin                []string
	ExternalOriginPrefix          string
	Origins                       map[string]string
	WildcardMapping               map[string]string
	CustomResponseTransformations [][]string
	ForwardReplacements           []string `json:"-"`
	BackwardReplacements          []string `json:"-"`
	LastForwardReplacements       []string `json:"-"`
	LastBackwardReplacements      []string `json:"-"`
	WildcardDomain                string   `json:"-"`

	// Ignore from JSON export
	loopCount int
	mu        sync.RWMutex
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
	for k, v := range newOrigins {
		r.Origins[k] = v
	}
}

// Save saves the Replacer struct to a file as JSON.
func (r *Replacer) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return saveToJSON(ReplaceFile, r)
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
	r.mu.Lock()
	mutex := r.mu
	defer mutex.Unlock()

	rep, err := loadFromJSON(ReplaceFile)
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
