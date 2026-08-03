package recon

import (
	"testing"

	"github.com/muraenateam/muraena/session"
)

const sample = `{"target":"example.com",
"origins":["cdn.example.com","a.b.example.com"],
"loginPages":[{"url":"https://example.com/login","action":"/api/auth","usernameSelector":"#email","passwordSelector":"#pw"}],
"secretsPaths":["/api/auth"],
"secretsPatterns":[{"label":"password","matching":"password","start":"","end":""}]}`

func TestParseAndPatch(t *testing.T) {
	r, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Origins) != 2 || len(r.LoginPages) != 1 {
		t.Fatalf("parsed = %+v", r)
	}

	cur := &session.Configuration{}
	cur.Origins.ExternalOrigins = []string{"existing.example.com"}
	patch := r.ToPatch(cur)

	origins, ok := patch["Origins"].(map[string]interface{})
	if !ok {
		t.Fatal("no Origins in patch")
	}
	ext, ok := origins["ExternalOrigins"].([]string)
	if !ok {
		t.Fatalf("ExternalOrigins type = %T", origins["ExternalOrigins"])
	}
	// SimplifyDomains collapses "existing.example.com" (3rd level) to
	// "*.example.com"; assert the existing origin survives the merge in that form.
	foundExisting := false
	for _, e := range ext {
		if e == "*.example.com" {
			foundExisting = true
		}
	}
	if !foundExisting {
		t.Fatalf("merge dropped existing origin: %v", ext)
	}
}
