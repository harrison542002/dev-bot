// Package llm provides a unified interface for multiple AI providers.
// Supported providers: claude, openai, gemini, local.
package llm

import "context"

// Usage holds token consumption for a single API call.
// It is nil for providers that do not report usage (e.g. local servers).
type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

// Client is the common interface all AI provider adapters implement.
type Client interface {
	// Complete sends a system prompt and a user message, and returns the model's
	// text response plus optional token usage (nil when unavailable).
	Complete(ctx context.Context, system, user string, maxTokens int) (text string, usage *Usage, err error)

	// ProviderName returns a human-readable label used in status messages.
	ProviderName() string
}

// ToolUser extends Client with multi-turn tool-use capability.
// Providers that implement this interface support an agentic loop where the
// model can call tools (read files, search code, write files, etc.) before
// producing a final answer.
type ToolUser interface {
	Client
	// CompleteWithTools runs one turn of the conversation. It returns a
	// Message that either contains tool calls (ToolUses non-empty) or a final
	// text reply (Text non-empty, ToolUses empty).
	CompleteWithTools(ctx context.Context, system string, messages []Message, tools []Tool, maxTokens int) (Message, *Usage, error)
}

// Tool is a function the model can invoke.
type Tool struct {
	Name        string
	Description string
	Parameters  ToolParameters
}

// ToolParameters is the JSON-schema–style description of a tool's inputs.
type ToolParameters struct {
	Properties map[string]ToolProperty
	Required   []string
}

// ToolProperty describes one parameter of a tool.
type ToolProperty struct {
	Type        string // "string" | "boolean" | "number"
	Description string
}

// Message is one turn in a multi-turn conversation.
type Message struct {
	Role        string       // "user" | "assistant"
	Text        string       // plain text content
	ToolUses    []ToolUse    // tool calls made by the assistant (assistant messages)
	ToolResults []ToolResult // executor results sent back (user messages)
}

// ToolUse is a tool call issued by the model.
type ToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResult is the executor's response to a ToolUse.
type ToolResult struct {
	ToolUseID string
	Content   string
	IsError   bool
}

// NativeAgent is implemented by providers that act as full coding agents
// rather than text-completion backends. When the agent detects this interface
// it delegates the entire code-writing, branching, pushing, and PR-creation
// workflow to the provider.
type NativeAgent interface {
	// RunAgent implements a task inside workDir. It must:
	//   1. Create a branch named branch
	//   2. Write the code required by title/description
	//   3. Commit and push the branch targeting baseBranch
	// It may also open the pull request itself; callers may create the PR
	// afterward when the branch has been pushed successfully.
	// ghToken is the GitHub Personal Access Token; pass it so the provider can
	// authenticate git or API calls when needed.
	RunAgent(ctx context.Context, workDir, branch, baseBranch, title, description, ghToken string) error
}
