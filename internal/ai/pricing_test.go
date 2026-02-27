package ai

import "testing"

func TestUsageCost(t *testing.T) {
	tests := []struct {
		name   string
		usage  Usage
		want   string
		wantOK bool
	}{
		{
			name:   "known model",
			usage:  Usage{Model: "gpt-4o-mini", InputTokens: 1000, OutputTokens: 50},
			want:   "$0.0002",
			wantOK: true,
		},
		{
			name:   "zero tokens",
			usage:  Usage{Model: "gpt-4o-mini", InputTokens: 0, OutputTokens: 0},
			want:   "$0.0000",
			wantOK: true,
		},
		{
			name:   "unknown model",
			usage:  Usage{Model: "llama3", InputTokens: 100, OutputTokens: 50},
			want:   "",
			wantOK: false,
		},
		{
			name:   "anthropic haiku",
			usage:  Usage{Model: "claude-haiku-4-5-20251001", InputTokens: 3000, OutputTokens: 30},
			want:   "$0.0032",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.usage.Cost()
			if ok != tt.wantOK {
				t.Errorf("Cost() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("Cost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		name  string
		usage Usage
		want  string
	}{
		{
			name:  "small counts",
			usage: Usage{InputTokens: 150, OutputTokens: 28},
			want:  "150 in / 28 out",
		},
		{
			name:  "large input",
			usage: Usage{InputTokens: 3100, OutputTokens: 28},
			want:  "3.1k in / 28 out",
		},
		{
			name:  "both large",
			usage: Usage{InputTokens: 5000, OutputTokens: 1200},
			want:  "5.0k in / 1.2k out",
		},
		{
			name:  "zero",
			usage: Usage{InputTokens: 0, OutputTokens: 0},
			want:  "0 in / 0 out",
		},
		{
			name:  "exactly 1000",
			usage: Usage{InputTokens: 1000, OutputTokens: 999},
			want:  "1.0k in / 999 out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.usage.FormatTokens()
			if got != tt.want {
				t.Errorf("FormatTokens() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModelInputCost(t *testing.T) {
	if got := ModelInputCost("gpt-4o-mini"); got != 0.15 {
		t.Errorf("ModelInputCost(gpt-4o-mini) = %v, want 0.15", got)
	}
	if got := ModelInputCost("nonexistent-model"); got != -1 {
		t.Errorf("ModelInputCost(nonexistent) = %v, want -1", got)
	}
}
