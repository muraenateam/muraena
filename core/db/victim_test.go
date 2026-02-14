package db

import (
	"testing"
	"time"
)

func TestNormalizeCookieExpiry(t *testing.T) {
	tests := []struct {
		name        string
		input       time.Time
		expectAfter time.Duration // result should be at least this far in the future
		expectExact bool          // if true, result should equal input exactly
	}{
		{
			name:        "Zero time gets normalized to ~48h from now",
			input:       time.Time{},
			expectAfter: 47 * time.Hour,
		},
		{
			name:        "Past time gets normalized to ~48h from now",
			input:       time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			expectAfter: 47 * time.Hour,
		},
		{
			name:        "Near future (<48h) gets normalized to ~48h from now",
			input:       time.Now().Add(1 * time.Hour),
			expectAfter: 47 * time.Hour,
		},
		{
			name:        "Far future (>48h) is preserved as-is",
			input:       time.Now().Add(720 * time.Hour), // 30 days
			expectExact: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeCookieExpiry(tt.input)

			if tt.expectExact {
				if !result.Equal(tt.input) {
					t.Errorf("Expected expiry to be preserved as %v, got %v", tt.input, result)
				}
				return
			}

			untilExpiry := time.Until(result)
			if untilExpiry < tt.expectAfter {
				t.Errorf("Expected expiry at least %v in the future, but got %v (expiry: %v)",
					tt.expectAfter, untilExpiry, result)
			}

			// Should not be more than 49 hours (48h + small margin for test execution)
			if untilExpiry > 49*time.Hour {
				t.Errorf("Expected expiry around 48h in the future, but got %v", untilExpiry)
			}
		})
	}
}
