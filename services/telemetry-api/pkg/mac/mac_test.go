package mac

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"AA:BB:CC:DD:EE:FF", "aabbccddeeff"},
		{"aa:bb:cc:dd:ee:ff", "aabbccddeeff"},
		{"AA-BB-CC-DD-EE-FF", "aabbccddeeff"},
		{"aa-bb-cc-dd-ee-ff", "aabbccddeeff"},
		{"AABB.CCDD.EEFF", "aabbccddeeff"},
		{"aabb.ccdd.eeff", "aabbccddeeff"},
		{"AABBCCDDEEFF", "aabbccddeeff"},
		{"aabbccddeeff", "aabbccddeeff"},
		{"Aa:Bb:Cc:Dd:Ee:Ff", "aabbccddeeff"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := Normalize(tc.input)
			if result != tc.expected {
				t.Errorf("Normalize(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"AA:BB:CC:DD:EE:FF", true},
		{"aa:bb:cc:dd:ee:ff", true},
		{"AA-BB-CC-DD-EE-FF", true},
		{"AABB.CCDD.EEFF", true},
		{"aabbccddeeff", true},
		{"AABBCCDDEEFF", true},

		// Invalid
		{"", false},
		{"AA:BB:CC:DD:EE", false},       // Too short
		{"AA:BB:CC:DD:EE:FF:00", false}, // Too long
		{"GG:HH:II:JJ:KK:LL", false},    // Invalid hex
		{"not-a-mac", false},
		{"12345", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := IsValid(tc.input)
			if result != tc.expected {
				t.Errorf("IsValid(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		input     string
		sep       string
		groupSize int
		expected  string
	}{
		{"aabbccddeeff", ":", 2, "aa:bb:cc:dd:ee:ff"},
		{"aabbccddeeff", "-", 2, "aa-bb-cc-dd-ee-ff"},
		{"aabbccddeeff", ".", 4, "aabb.ccdd.eeff"},
		{"AA:BB:CC:DD:EE:FF", ":", 2, "aa:bb:cc:dd:ee:ff"},

		// Invalid MAC - returns original
		{"invalid", ":", 2, "invalid"},
		{"short", ":", 2, "short"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := Format(tc.input, tc.sep, tc.groupSize)
			if result != tc.expected {
				t.Errorf("Format(%q, %q, %d) = %q, want %q", tc.input, tc.sep, tc.groupSize, result, tc.expected)
			}
		})
	}
}

func TestToColonFormat(t *testing.T) {
	result := ToColonFormat("aabbccddeeff")
	expected := "AA:BB:CC:DD:EE:FF"
	if result != expected {
		t.Errorf("ToColonFormat() = %q, want %q", result, expected)
	}
}

func TestToHyphenFormat(t *testing.T) {
	result := ToHyphenFormat("aabbccddeeff")
	expected := "AA-BB-CC-DD-EE-FF"
	if result != expected {
		t.Errorf("ToHyphenFormat() = %q, want %q", result, expected)
	}
}

func TestToDotFormat(t *testing.T) {
	result := ToDotFormat("aabbccddeeff")
	expected := "AABB.CCDD.EEFF"
	if result != expected {
		t.Errorf("ToDotFormat() = %q, want %q", result, expected)
	}
}
