// Package mac provides utilities for working with MAC addresses.
package mac

import (
	"regexp"
	"strings"
)

var validMAC = regexp.MustCompile("^[0-9a-f]{12}$")

// Normalize converts a MAC address to a canonical lowercase format without separators.
// Accepts common formats: AA:BB:CC:DD:EE:FF, aa-bb-cc-dd-ee-ff, aabb.ccdd.eeff, aabbccddeeff
func Normalize(mac string) string {
	// Remove all common separators (colons, hyphens, dots)
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	mac = strings.ReplaceAll(mac, ".", "")
	return strings.ToLower(mac)
}

// IsValid checks if a string is a valid MAC address (after normalization).
// Returns true for any format that normalizes to exactly 12 hex characters.
func IsValid(mac string) bool {
	return validMAC.MatchString(Normalize(mac))
}

// Format converts a normalized MAC address to a specific format.
// sep is the separator to use (e.g., ":", "-", ".")
// groupSize is how many characters between separators (2 for AA:BB:CC, 4 for AABB.CCDD)
func Format(mac string, sep string, groupSize int) string {
	normalized := Normalize(mac)
	if len(normalized) != 12 {
		return mac // Return original if invalid
	}

	var parts []string
	for i := 0; i < 12; i += groupSize {
		end := i + groupSize
		if end > 12 {
			end = 12
		}
		parts = append(parts, normalized[i:end])
	}
	return strings.Join(parts, sep)
}

// ToColonFormat converts to AA:BB:CC:DD:EE:FF format (uppercase with colons).
func ToColonFormat(mac string) string {
	return strings.ToUpper(Format(mac, ":", 2))
}

// ToHyphenFormat converts to AA-BB-CC-DD-EE-FF format (uppercase with hyphens).
func ToHyphenFormat(mac string) string {
	return strings.ToUpper(Format(mac, "-", 2))
}

// ToDotFormat converts to AABB.CCDD.EEFF format (uppercase with dots, Cisco style).
func ToDotFormat(mac string) string {
	return strings.ToUpper(Format(mac, ".", 4))
}
