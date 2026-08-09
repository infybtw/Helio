package commands

import "testing"

func TestParseDurationRequiresPositiveDuration(t *testing.T) {
	for _, input := range []string{"0s", "-1m", "0d", "-1d", "1d-24h"} {
		if _, err := ParseDuration(input); err == nil {
			t.Errorf("ParseDuration(%q) succeeded; want an error", input)
		}
	}
}

func TestParseDurationAcceptsPositiveDuration(t *testing.T) {
	for _, input := range []string{"30", "30m", "1h30m", "2d12h"} {
		if duration, err := ParseDuration(input); err != nil || duration <= 0 {
			t.Errorf("ParseDuration(%q) = %v, %v; want positive duration", input, duration, err)
		}
	}
}
