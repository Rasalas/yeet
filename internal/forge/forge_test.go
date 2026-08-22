package forge

import (
	"errors"
	"testing"
)

func overrideForgeSeams(t *testing.T, remote string, remoteErr error, look map[string]bool, runner func(name string, args ...string) (string, error)) {
	t.Helper()

	origRemote, origLook, origRun := remoteURL, lookPath, run
	t.Cleanup(func() { remoteURL, lookPath, run = origRemote, origLook, origRun })

	remoteURL = func() (string, error) { return remote, remoteErr }
	lookPath = func(name string) (string, error) {
		if look[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	if runner != nil {
		run = runner
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		remote    string
		available []string
		want      Forge
		wantErr   bool
	}{
		{
			name:      "gitlab remote with glab",
			remote:    "git@gitlab.com:team/repo.git",
			available: []string{"glab", "gh"},
			want:      GitLab{},
		},
		{
			name:      "unknown host with gh and glab prefers github",
			remote:    "git@git.mycorp.com:team/repo.git",
			available: []string{"glab", "gh"},
			want:      GitHub{},
		},
		{
			name:      "gitlab remote without glab",
			remote:    "git@gitlab.com:team/repo.git",
			available: []string{"gh"},
			wantErr:   true,
		},
		{
			name:      "github remote prefers gh",
			remote:    "git@github.com:team/repo.git",
			available: []string{"gh"},
			want:      GitHub{},
		},
		{
			name:      "unknown host with only glab assumes gitlab",
			remote:    "git@git.mycorp.com:team/repo.git",
			available: []string{"glab"},
			want:      GitLab{},
		},
		{
			name:      "unknown host without any cli",
			remote:    "git@github.com:team/repo.git",
			available: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := map[string]bool{}
			for _, name := range tt.available {
				lookup[name] = true
			}
			overrideForgeSeams(t, tt.remote, nil, lookup, nil)

			got, err := Detect()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Detect() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			switch tt.want.(type) {
			case GitLab:
				if _, ok := got.(GitLab); !ok {
					t.Errorf("Detect() = %T, want %T", got, tt.want)
				}
			case GitHub:
				if _, ok := got.(GitHub); !ok {
					t.Errorf("Detect() = %T, want %T", got, tt.want)
				}
			}
		})
	}
}

func TestGitHubExistingPR(t *testing.T) {
	var calls int
	overrideForgeSeams(t, "", nil, nil, func(name string, args ...string) (string, error) {
		calls++
		return "https://github.com/team/repo/pull/7\n", nil
	})

	url, exists := (GitHub{}).ExistingPR("feat/x")
	if !exists || url != "https://github.com/team/repo/pull/7" {
		t.Errorf("ExistingPR() = %q, %v", url, exists)
	}
	if calls != 1 {
		t.Errorf("cli calls = %d, want 1", calls)
	}

	overrideForgeSeams(t, "", nil, nil, func(name string, args ...string) (string, error) {
		return "", errors.New("exit status 1")
	})
	if _, exists := (GitHub{}).ExistingPR("feat/x"); exists {
		t.Error("ExistingPR() reported existing on CLI failure")
	}
}

func TestGitLabExistingPRParsesJSON(t *testing.T) {
	tests := []struct {
		name          string
		jsonOut       string
		jsonErr       error
		wantURL       string
		wantExists    bool
		wantJSONCalls int // calls with --output json before falling back
	}{
		{
			name:          "web url extracted",
			jsonOut:       `{"web_url":"https://gitlab.com/team/repo/-/merge_requests/3","iid":3}`,
			wantURL:       "https://gitlab.com/team/repo/-/merge_requests/3",
			wantExists:    true,
			wantJSONCalls: 1,
		},
		{
			name:          "empty object means no MR",
			jsonOut:       `{}`,
			wantJSONCalls: 1,
		},
		{
			name:          "json null means no MR",
			jsonOut:       `null`,
			wantJSONCalls: 1,
		},
		{
			name:          "command error falls back to plain view",
			jsonErr:       errors.New("exit status 1"),
			wantJSONCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var jsonCalls int
			overrideForgeSeams(t, "", nil, nil, func(name string, args ...string) (string, error) {
				for _, a := range args {
					if a == "--output" {
						jsonCalls++
						return tt.jsonOut, tt.jsonErr
					}
				}
				// Plain-view fallback: no open MR for this branch.
				return "no open merge request for feat/x\n", errors.New("exit status 1")
			})

			url, exists := (GitLab{}).ExistingPR("feat/x")
			if exists != tt.wantExists || url != tt.wantURL {
				t.Errorf("ExistingPR() = %q, %v; want %q, %v", url, exists, tt.wantURL, tt.wantExists)
			}
			if jsonCalls != tt.wantJSONCalls {
				t.Errorf("json cli calls = %d, want %d", jsonCalls, tt.wantJSONCalls)
			}
		})
	}
}

func TestGitLabExistingPRFallsBackToPlainOutput(t *testing.T) {
	overrideForgeSeams(t, "", nil, nil, func(name string, args ...string) (string, error) {
		if len(args) >= 2 && args[1] == "--output" {
			return "--output is not supported by this glab", errors.New("flag error")
		}
		return "!3 feat: something\n\nurl: https://gitlab.com/team/repo/-/merge_requests/3\n", nil
	})

	url, exists := (GitLab{}).ExistingPR("feat/x")
	if !exists || url != "https://gitlab.com/team/repo/-/merge_requests/3" {
		t.Errorf("ExistingPR() = %q, %v", url, exists)
	}
}
