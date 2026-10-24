package compressor

import (
	"fmt"
	"strconv"
	"strings"
)

// LogIDCompressor is a highly efficient bidirectional mapper for compressing long log IDs (e.g., UUIDs)
// into sequential tokens (e.g., "log-0001") for LLM inference.
// Note: This struct is not thread-safe. Wrap with a sync.Mutex if accessed concurrently.
type LogIDCompressor struct {
	prefix          string
	originalToShort map[string]string
	originalIDs     []string
}

// NewLogIDCompressor creates a new compressor with the given prefix.
func NewLogIDCompressor(prefix string) *LogIDCompressor {
	return &LogIDCompressor{
		prefix:          prefix,
		originalToShort: make(map[string]string),
		originalIDs:     make([]string, 0),
	}
}

// Size returns the number of distinct original IDs compressed so far.
func (c *LogIDCompressor) Size() int {
	return len(c.originalIDs)
}

// Compress converts a long ID into a prefixed 4-digit short ID (e.g., "log-0001").
// If the ID has already been seen, it returns the existing short ID.
func (c *LogIDCompressor) Compress(originalID string) string {
	if existing, exists := c.originalToShort[originalID]; exists {
		return existing
	}

	c.originalIDs = append(c.originalIDs, originalID)
	nextNumber := len(c.originalIDs) // 1-based length

	shortID := fmt.Sprintf("%s%04d", c.prefix, nextNumber)

	c.originalToShort[originalID] = shortID
	return shortID
}

// Decompress recovers the original ID from a short ID.
// Returns the original ID and true if successful, or an empty string and false if invalid.
func (c *LogIDCompressor) Decompress(shortID string) (string, bool) {
	if c.prefix != "" && !strings.HasPrefix(shortID, c.prefix) {
		return "", false
	}

	numStr := strings.TrimPrefix(shortID, c.prefix)
	nextNumber, err := strconv.Atoi(numStr)
	if err != nil {
		return "", false
	}

	index := nextNumber - 1
	if index < 0 || index >= len(c.originalIDs) {
		return "", false
	}

	return c.originalIDs[index], true
}

// Reset clears the internal state.
// Reuses the underlying array capacity and map allocation for efficiency across Object Pools.
func (c *LogIDCompressor) Reset() {
	clear(c.originalToShort)          // Requires Go 1.21+
	c.originalIDs = c.originalIDs[:0] // Reslices to 0 length while keeping allocated capacity
}
