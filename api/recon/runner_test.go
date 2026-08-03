//go:build !windows

package recon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFakeNode creates a shell script masquerading as "node" that echoes a
// fixed recon JSON to stdout, so the runner can be tested without puppeteer.
func writeFakeNode(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "fake.js") // ignored by our fake node
	node := filepath.Join(dir, "node")
	body := "#!/bin/sh\ncat <<'JSON'\n" + sample + "\nJSON\n"
	if err := os.WriteFile(node, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return node, script
}

func TestRunnerHappyPath(t *testing.T) {
	node, script := writeFakeNode(t)
	r := NewRunner()
	var lines []string
	id, err := r.Start(node, script, "https://example.com", 1, 5, func(s string) { lines = append(lines, s) })
	if err != nil {
		t.Fatal(err)
	}
	// wait for completion
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if j, _ := r.Get(id); j != nil && j.Status != "running" {
			if j.Status != "done" {
				t.Fatalf("status = %s err=%s", j.Status, j.Err)
			}
			if j.Result == nil || len(j.Result.Origins) == 0 {
				t.Fatal("no result parsed")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("job did not complete")
}

func TestRunnerRejectsBadTarget(t *testing.T) {
	r := NewRunner()
	if _, err := r.Start("node", "x.js", "not-a-url", 1, 5, nil); err == nil {
		t.Fatal("expected error for invalid target")
	}
}
