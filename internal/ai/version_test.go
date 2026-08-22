package ai

import "testing"

func TestSetClientVersion(t *testing.T) {
	orig := ClientVersion()
	t.Cleanup(func() { SetClientVersion(orig) })

	SetClientVersion("v1.2.3")
	if got := ClientVersion(); got != "v1.2.3" {
		t.Errorf("ClientVersion() = %q, want %q", got, "v1.2.3")
	}

	// Empty values must not clobber the injected version — callers pass
	// ldflags variables blindly.
	SetClientVersion("")
	if got := ClientVersion(); got != "v1.2.3" {
		t.Errorf("ClientVersion() = %q after empty set, want %q", got, "v1.2.3")
	}
}
