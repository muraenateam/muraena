package tracking

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/core/db"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/module/telegram"
	"github.com/muraenateam/muraena/session"
)

var m *Tracker

// init test
func init() {
	debug := true
	verbose := false
	log.Init(core.Options{Debug: &debug, Verbose: &verbose}, false, "")

	s := &session.Session{}
	s.Config = &session.Configuration{}

	m = &Tracker{
		SessionModule: session.NewSessionModule(Name, s),
		Enabled:       true,
	}

	s.Register(&telegram.Telegram{
		SessionModule: session.NewSessionModule(telegram.Name, s),
		Enabled:       true,
		BotToken:      "1587304999:AAG4cH8VzJ1b8tbamq0VZM9C01KkDjY5IFo",
		ChatID:        []string{"-1001856562703"},
	}, nil)

	s.InitRedis()
}

// TestModuleName ensures the module is the same, just in case :)
func TestModuleName(t *testing.T) {
	module := "tracking"
	want := regexp.MustCompile(Name)
	if !want.MatchString(Name) {
		t.Fatalf(`The module name does not match: %q != %q`, module, want)
	}
}

// TestModuleName ensures the module is the same, just in case :)
func TestPushVictim(t *testing.T) {

	v := &db.Victim{
		ID:           "AAAAA",
		IP:           "192.157.1.1",
		UA:           "Parakalo file mou",
		RequestCount: 0,
		FirstSeen:    time.Now().UTC().Format("2006-01-02 15:04:05"),
		LastSeen:     time.Now().UTC().Format("2006-01-02 15:04:05"),
	}

	m.PushVictim(v)
}

// TestTrackingIdentifierExtraction tests that tracking IDs are properly extracted from query parameters
func TestTrackingIdentifierExtraction(t *testing.T) {
	s := &session.Session{}
	s.Config = &session.Configuration{}

	// Configure tracking with a simple 6-digit numeric validator
	s.Config.Tracking.Enabled = true
	s.Config.Tracking.Trace.Identifier = "id"
	s.Config.Tracking.Trace.ValidatorRegex = "[0-9]{6}"
	s.Config.Tracking.Trace.Landing.Type = "query"

	tracker, err := Load(s)
	if err != nil {
		t.Fatalf("Failed to load tracker: %v", err)
	}

	s.InitRedis()

	tests := []struct {
		name          string
		url           string
		expectedID    string
		shouldBeValid bool
		description   string
	}{
		{
			name:          "Valid 6-digit ID in query",
			url:           "https://example.com/page?id=123456",
			expectedID:    "123456",
			shouldBeValid: true,
			description:   "Should extract and use the valid 6-digit ID from query parameter",
		},
		{
			name:          "Invalid 5-digit ID in query",
			url:           "https://example.com/page?id=12345",
			expectedID:    "",
			shouldBeValid: false,
			description:   "Should return invalid trace for 5-digit ID",
		},
		{
			name:          "Invalid 7-digit ID in query",
			url:           "https://example.com/page?id=1234567",
			expectedID:    "",
			shouldBeValid: false,
			description:   "Should return invalid trace for 7-digit ID",
		},
		{
			name:          "No ID in query",
			url:           "https://example.com/page",
			expectedID:    "",
			shouldBeValid: false,
			description:   "Should return invalid trace when no ID present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.url, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			trace := tracker.TrackRequest(req)

			if tt.shouldBeValid {
				if trace.ID != tt.expectedID {
					t.Errorf("Expected ID %s but got %s. %s", tt.expectedID, trace.ID, tt.description)
				}
				if !trace.IsValid() {
					t.Errorf("Expected trace to be valid but it wasn't. ID: %s", trace.ID)
				}
			} else {
				// Without auto-generation, invalid/missing IDs should return an invalid trace
				if trace.IsValid() {
					t.Errorf("Expected invalid trace but got valid one with ID: %s. %s", trace.ID, tt.description)
				}
			}
		})
	}
}

// TestValidatorRegexAnchoring tests that validator regex properly uses anchors
func TestValidatorRegexAnchoring(t *testing.T) {
	s := &session.Session{}
	s.Config = &session.Configuration{}

	s.Config.Tracking.Enabled = true
	s.Config.Tracking.Trace.Identifier = "id"
	s.Config.Tracking.Trace.ValidatorRegex = "[0-9]{6}" // Without anchors in config

	tracker, err := Load(s)
	if err != nil {
		t.Fatalf("Failed to load tracker: %v", err)
	}

	// The validator regex should have been automatically anchored
	expectedRegex := "^[0-9]{6}$"
	if tracker.ValidatorRegex.String() != expectedRegex {
		t.Errorf("Expected validator regex to be %s but got %s", expectedRegex, tracker.ValidatorRegex.String())
	}

	// Test that it properly validates only exact matches
	trace := tracker.makeTrace("123456")
	if !trace.IsValid() {
		t.Errorf("Expected '123456' to be valid")
	}

	// Should reject IDs that contain the pattern but have extra characters
	trace = tracker.makeTrace("123456789") // 9 digits
	if trace.IsValid() {
		t.Errorf("Expected '123456789' to be invalid (too long)")
	}

	trace = tracker.makeTrace("abc123456") // letters + 6 digits
	if trace.IsValid() {
		t.Errorf("Expected 'abc123456' to be invalid (has letters)")
	}

	trace = tracker.makeTrace("123456abc") // 6 digits + letters
	if trace.IsValid() {
		t.Errorf("Expected '123456abc' to be invalid (has letters)")
	}
}
