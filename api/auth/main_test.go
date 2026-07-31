package auth

import (
	"os"
	"testing"

	"github.com/muraenateam/muraena/log"
)

// TestMain registers a log output. The muraena log package panics when a log
// function is called with no output configured (Bootstrap logs on success).
func TestMain(m *testing.M) {
	_ = log.AddOutput("", log.INFO, log.FormatConfigBasic, true)
	os.Exit(m.Run())
}
