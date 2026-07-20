package prompts

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

type Rendered struct {
	Path    string
	Content []byte
}

// Render produces runner discovery assets from one canonical prompt.
func Render(prompt Prompt) ([]Rendered, error) {
	id := string(prompt.ID)
	switch {
	case prompt.ID == Orchestrator:
		return []Rendered{{Path: "ORCHESTRATOR.md", Content: prompt.Content}}, nil
	case prompt.ID == Assistant:
		return []Rendered{{Path: "ASSISTANT.md", Content: prompt.Content}}, nil
	case strings.HasPrefix(id, "skills/"):
		return []Rendered{{Path: id + "/SKILL.md", Content: prompt.Content}}, nil
	case strings.HasPrefix(id, "agents/"):
		name := path.Base(id)
		codex, err := renderCodexAgent(prompt.Content)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompt.ID, err)
		}
		return []Rendered{
			{Path: ".claude/agents/" + name + ".md", Content: prompt.Content},
			{Path: ".codex/agents/" + name + ".toml", Content: codex},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported prompt id %q", prompt.ID)
	}
}

func renderCodexAgent(content []byte) ([]byte, error) {
	text := string(content)
	if !strings.HasPrefix(text, "---\n") {
		return nil, fmt.Errorf("agent prompt has no frontmatter")
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return nil, fmt.Errorf("agent prompt has unterminated frontmatter")
	}
	end += 4
	metadata := make(map[string]string)
	for _, line := range strings.Split(text[4:end], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			metadata[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	name, description := metadata["name"], metadata["description"]
	if name == "" || description == "" {
		return nil, fmt.Errorf("agent prompt requires name and description")
	}
	body := strings.TrimSpace(text[end+len("\n---\n"):])
	var out strings.Builder
	fmt.Fprintf(&out, "name = %s\ndescription = %s\n", strconv.Quote(name), strconv.Quote(description))
	if sandbox := codexSandbox(name); sandbox != "" {
		fmt.Fprintf(&out, "sandbox_mode = %s\n", strconv.Quote(sandbox))
	}
	out.WriteString("developer_instructions = \"\"\"\n")
	out.WriteString(strings.ReplaceAll(body, "\"\"\"", "\\\"\\\"\\\""))
	out.WriteString("\n\"\"\"\n")
	return []byte(out.String()), nil
}

func codexSandbox(name string) string {
	switch name {
	case "code-reviewer", "codebase-researcher", "external-researcher", "pattern-finder", "qrspi-researcher", "thoughts-researcher":
		return "read-only"
	case "qrspi-research-lead", "test-verifier":
		return "workspace-write"
	default:
		return ""
	}
}
