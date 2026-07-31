package session

import "testing"

func TestDeepMergePreservesSiblings(t *testing.T) {
	dst := map[string]interface{}{
		"Proxy": map[string]interface{}{"Phishing": "a", "Port": 443},
	}
	src := map[string]interface{}{
		"Proxy": map[string]interface{}{"Port": 8443},
	}
	out := DeepMerge(dst, src)
	p := out["Proxy"].(map[string]interface{})
	if p["Phishing"] != "a" {
		t.Fatal("sibling Phishing lost")
	}
	if int(p["Port"].(int)) != 8443 {
		t.Fatalf("Port = %v, want 8443", p["Port"])
	}
}

func TestBuildCandidateClassifiesRestart(t *testing.T) {
	s := &Session{}
	c := &Configuration{}
	c.Proxy.Phishing = "a.example"
	c.Transform.Request.UserAgent = "old"
	s.SwapConfig(c)

	// live field only
	cand, rr, err := s.BuildCandidate(map[string]interface{}{
		"Transform": map[string]interface{}{"Request": map[string]interface{}{"UserAgent": "new"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rr) != 0 {
		t.Fatalf("unexpected restart fields %v", rr)
	}
	if cand.Transform.Request.UserAgent != "new" {
		t.Fatalf("UserAgent not merged: %q", cand.Transform.Request.UserAgent)
	}
	if cand.Proxy.Phishing != "a.example" {
		t.Fatal("Phishing lost in candidate")
	}

	// restart field
	_, rr, err = s.BuildCandidate(map[string]interface{}{
		"Proxy": map[string]interface{}{"Port": 9999},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rr) != 1 || rr[0] != "Proxy.Port" {
		t.Fatalf("restart fields = %v", rr)
	}
}
