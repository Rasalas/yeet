package ai

// clientVersion is reported to ACP agents during protocol initialization
// so adapters can log/debug client compatibility. Defaults to "dev";
// release builds inject the real version via main.version.
var clientVersion = "dev"

// SetClientVersion records the build version reported to ACP agents.
// Empty values are ignored so callers can pass ldflags variables blindly.
func SetClientVersion(v string) {
	if v != "" {
		clientVersion = v
	}
}

// ClientVersion returns the version reported to ACP agents.
func ClientVersion() string {
	return clientVersion
}
