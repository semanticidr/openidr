package compressor

import (
	"fmt"
)

// GlobalFieldCompressor extracts fields globally if they are uniform across the payload.
// Note: This struct is not thread-safe. Wrap with a sync.Mutex if accessed concurrently.
type GlobalFieldCompressor struct {
	inlinePrefix string
	values       map[string]struct{}
	isMixed      bool
}

// NewGlobalFieldCompressor creates a new compressor with the given inline prefix.
func NewGlobalFieldCompressor(inlinePrefix string) *GlobalFieldCompressor {
	return &GlobalFieldCompressor{
		inlinePrefix: inlinePrefix,
		values:       make(map[string]struct{}),
	}
}

// Track records a value and determines if the field has mixed values.
func (c *GlobalFieldCompressor) Track(value string) {
	if c.isMixed {
		return
	}

	// struct{}{} uses 0 bytes of memory
	c.values[value] = struct{}{}

	if len(c.values) > 1 {
		c.isMixed = true
	}
}

// GetGlobalHeader returns the formatted header if all tracked values are identical.
// Returns the header string and true if uniform, or an empty string and false if mixed or empty.
func (c *GlobalFieldCompressor) GetGlobalHeader(headerLabel string) (string, bool) {
	if !c.isMixed && len(c.values) == 1 {
		// Extract the single value by iterating (it will only loop once)
		for val := range c.values {
			return fmt.Sprintf("%s: %s", headerLabel, val), true
		}
	}
	return "", false
}

// Compress returns an inline formatted value if the field is mixed across the payload.
// If uniform, it returns an empty string (meaning it can be safely omitted inline).
func (c *GlobalFieldCompressor) Compress(value string) string {
	if !c.isMixed {
		return ""
	}
	// Using fmt.Sprintf to mirror template literals, but string concatenation
	// (c.inlinePrefix + "=" + value + " ") would be slightly faster.
	return fmt.Sprintf("%s=%s ", c.inlinePrefix, value)
}

// Reset clears the internal state for reuse.
// Reuses the underlying map allocation for efficiency across Object Pools.
func (c *GlobalFieldCompressor) Reset() {
	clear(c.values) // Requires Go 1.21+
	c.isMixed = false
}
