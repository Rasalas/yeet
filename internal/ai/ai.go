package ai

// Provider generates a commit message from git context.
type Provider interface {
	GenerateCommitMessage(ctx CommitContext) (string, Usage, error)
}

// Usage holds token counts and model info for cost reporting.
type Usage struct {
	Model        string
	InputTokens  int
	OutputTokens int
}
