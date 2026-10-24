package compressor

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

const (
	nanosPerSecond = 1_000_000_000
)

// DynamicTimestampCompressor anchors the timeline to a specific T0 (trigger log)
// and compresses all other timestamps into relative, bucketed seconds.
// Note: This struct is not thread-safe. Wrap with a sync.Mutex if accessed concurrently.
type DynamicTimestampCompressor struct {
	t0          int64
	hasT0       bool
	windowNanos int64
	windowLabel string
}

// NewDynamicTimestampCompressor initializes a new compressor with default settings.
func NewDynamicTimestampCompressor() *DynamicTimestampCompressor {
	return &DynamicTimestampCompressor{
		windowNanos: nanosPerSecond,
		windowLabel: "1s",
	}
}

// Analyze performs a fast pre-pass to determine the total spread of events
// and sets the ideal concurrency window.
func (c *DynamicTimestampCompressor) Analyze(triggerT0Str string, allTimestamps []string) error {
	t0, err := strconv.ParseInt(triggerT0Str, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid T0 timestamp: %w", err)
	}

	c.t0 = t0
	c.hasT0 = true

	minTime, maxTime := t0, t0

	for _, tsStr := range allTimestamps {
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			// Skip invalid timestamps just like the JS loop would likely NaN out or throw
			continue
		}
		if ts < minTime {
			minTime = ts
		}
		if ts > maxTime {
			maxTime = ts
		}
	}

	spreadNanos := maxTime - minTime

	if spreadNanos <= 1_000_000_000 { // <= 1 second
		c.windowNanos = 10_000_000
		c.windowLabel = "10ms"
	} else if spreadNanos <= 10_000_000_000 { // <= 10 seconds
		c.windowNanos = 100_000_000
		c.windowLabel = "100ms"
	} else if spreadNanos <= 60_000_000_000 { // <= 1 minute
		c.windowNanos = 500_000_000
		c.windowLabel = "500ms"
	} else {
		c.windowNanos = 1_000_000_000
		c.windowLabel = "1s"
	}

	return nil
}

// Compress buckets a timestamp into a relative format (e.g., "-0.5s", "+2s")
func (c *DynamicTimestampCompressor) Compress(nanosStr string) string {
	if !c.hasT0 {
		return "+0s"
	}

	current, err := strconv.ParseInt(nanosStr, 10, 64)
	if err != nil {
		return "+0s" // fallback
	}

	rawDeltaNanos := current - c.t0

	// Go integer division inherently truncates towards zero (like JS BigInt).
	bucketedDeltaNanos := (rawDeltaNanos / c.windowNanos) * c.windowNanos

	// Convert nanoseconds to decimal seconds.
	seconds := math.Abs(float64(bucketedDeltaNanos) / float64(nanosPerSecond))

	// Determine the sign based on the raw delta, not the bucket.
	sign := "-"
	if rawDeltaNanos >= 0 {
		sign = "+"
	}

	// %g formats floats without trailing zeros (e.g., 0.5 instead of 0.500)
	return fmt.Sprintf("%s%gs", sign, seconds)
}

// GetHeaderInstruction generates the system instruction for the LLM prompt header.
func (c *DynamicTimestampCompressor) GetHeaderInstruction() string {
	t0Iso := "unknown"
	if c.hasT0 {
		// Matches the JS `toISOString()` format exactly
		t0Iso = time.Unix(0, c.t0).UTC().Format("2006-01-02T15:04:05.000Z")
	}

	return fmt.Sprintf("Timeline anchored at T0=%s (Trigger Event). Concurrency window: %s. Logs sharing the exact same 'ts' value occurred concurrently; rely strictly on their top-to-bottom order for micro-sequence.", t0Iso, c.windowLabel)
}

// ResolveOffsetToISO resolves a millisecond offset from T0 into an absolute ISO timestamp.
// Returns the timestamp and true if successful, or an empty string and false if T0 isn't set.
func (c *DynamicTimestampCompressor) ResolveOffsetToISO(offsetMs int64) (string, bool) {
	if !c.hasT0 {
		return "", false
	}

	targetNanos := c.t0 + (offsetMs * 1_000_000)
	return time.Unix(0, targetNanos).UTC().Format("2006-01-02T15:04:05.000Z"), true
}

// Reset clears the internal state for reuse in Object Pools.
func (c *DynamicTimestampCompressor) Reset() {
	c.hasT0 = false
	c.windowNanos = nanosPerSecond
	c.windowLabel = "1s"
}
