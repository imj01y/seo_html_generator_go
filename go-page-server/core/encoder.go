package core

import (
	"fmt"
	"math/rand"
	"strings"
)

// HTMLEntityEncoder encodes non-ASCII characters to HTML entities
type HTMLEntityEncoder struct {
	mixRatio float64 // Ratio of hex encoding (0.5 = 50% hex, 50% decimal)
}

// NewHTMLEntityEncoder creates a new encoder with the specified mix ratio
func NewHTMLEntityEncoder(mixRatio float64) *HTMLEntityEncoder {
	return &HTMLEntityEncoder{
		mixRatio: mixRatio,
	}
}

// EncodeText encodes non-ASCII characters in the text to HTML entities
// ASCII characters (0-127) are preserved as-is
func (e *HTMLEntityEncoder) EncodeText(text string) string {
	if text == "" {
		return ""
	}

	var sb strings.Builder
	sb.Grow(len(text) * 2) // Pre-allocate for efficiency

	for _, r := range text {
		if r <= 127 {
			// ASCII character, keep as-is
			sb.WriteRune(r)
		} else {
			// Non-ASCII character, encode
			if rand.Float64() < e.mixRatio {
				// Hex encoding: &#x数字;
				sb.WriteString(fmt.Sprintf("&#x%x;", r))
			} else {
				// Decimal encoding: &#数字;
				sb.WriteString(fmt.Sprintf("&#%d;", r))
			}
		}
	}

	return sb.String()
}

// Encode is an alias for EncodeText
func (e *HTMLEntityEncoder) Encode(text string) string {
	return e.EncodeText(text)
}

// Global encoder instance
var globalEncoder *HTMLEntityEncoder

// InitEncoder initializes the global encoder
func InitEncoder(mixRatio float64) {
	globalEncoder = NewHTMLEntityEncoder(mixRatio)
}

// GetEncoder returns the global encoder
func GetEncoder() *HTMLEntityEncoder {
	if globalEncoder == nil {
		globalEncoder = NewHTMLEntityEncoder(0.5)
	}
	return globalEncoder
}

// Encode is a convenience function that uses the global encoder
func Encode(text string) string {
	return GetEncoder().EncodeText(text)
}
