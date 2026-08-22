package cmd

import "time"

// streamFrameInterval caps streaming preview redraws at ~30 fps. Fast
// providers emit many small tokens; redrawing the whole card per token
// flickers and does quadratic rewrap work for long messages.
const streamFrameInterval = 33 * time.Millisecond

// renderThrottle decides whether a preview redraw is due. The zero value is
// ready to use.
type renderThrottle struct {
	last time.Time
}

// due reports whether enough time has passed since the last allowed render,
// recording now as the new reference when it returns true.
func (t *renderThrottle) due(now time.Time) bool {
	if t.last.IsZero() || now.Sub(t.last) >= streamFrameInterval {
		t.last = now
		return true
	}
	return false
}
