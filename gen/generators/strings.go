// Package generators provides value generators for adversarial and property-style testing.
package generators

import "strings"

// Strings returns a deterministic slice of adversarial string inputs for fuzz-style testing.
func Strings() []string {
	const longSize = 10*1024 + 1 // > 10k
	return []string{
		"",
		" ",
		"\n",
		"\x00",
		"invalid",
		"v",
		"v1",
		"v1.2",
		"v1.2.3",
		strings.Repeat("x", longSize),
	}
}

// Empty returns just the empty string.
func Empty() []string { return []string{""} }

// Whitespace returns whitespace-only strings in deterministic order.
func Whitespace() []string { return []string{" ", "\t", "\n", "\r\n", " \t\n "} }

// InvalidUTF8 returns strings that are not valid UTF-8.
func InvalidUTF8() []string { return []string{"\xff\xfe", "\x80\x81\x82", "\xc0\x80"} }

// VeryLong returns strings longer than 10k characters.
func VeryLong() []string { return []string{strings.Repeat("a", 10*1024+1)} }
