package cmd

import (
	"testing"
	"time"
)

func TestRenderThrottle(t *testing.T) {
	var th renderThrottle
	base := time.Unix(0, 0)

	if !th.due(base) {
		t.Error("first render must be due")
	}
	if th.due(base.Add(10 * time.Millisecond)) {
		t.Error("render within the frame interval must be throttled")
	}
	if th.due(base.Add(streamFrameInterval - time.Millisecond)) {
		t.Error("render just before the interval must be throttled")
	}
	if !th.due(base.Add(streamFrameInterval)) {
		t.Error("render at the interval boundary must be due")
	}
}
