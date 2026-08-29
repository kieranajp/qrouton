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
		return []Rendered{{Path: OrchestratorAsset, Content: prompt.Content}}, nil
	case prompt.ID == Assistant:
		return []Rendered{{Path: AssistantAsset, Content: prompt.Content}}, nil
	case strings.HasPrefix(id, skillIDPrefix):
		out := []Rendered{{Path: id + "/" + skillFileName, Content: prompt.Content}}
		for _, file := range prompt.Files {
			out = append(out, Rendered{Path: id + "/" + file.Path, Content: file.Content})
		}
		return out, nil
	case strings.HasPrefix(id, agentIDPrefix):
		name := path.Base(id)
		codex, err := renderCodexAgent(prompt.Content)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompt.ID, err)
		}
		return []Rendered{
			{Path: claudeAgentsDir + name + promptFileExt, Content: prompt.Content},
			{Path: codexAgentsDir + name + tomlExtension, Content: codex},
		}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedPrompt, prompt.ID)
	}
}

func renderCodexAgent(content []byte) ([]byte, error) {
	text := string(content)
	if !strings.HasPrefix(text, frontmatterFence) {
		return nil, ErrNoFrontmatter
	}
	end := strings.Index(text[len(frontmatterFence):], frontmatterClose)
	if end < 0 {
		return nil, ErrUnterminatedFrontmatter
	}
	end += len(frontmatterFence)
	metadata := make(map[string]string)
	for _, line := range strings.Split(text[len(frontmatterFence):end], "\n") {
		key, value, ok := strings.Cut(line, frontmatterKeySep)
		if ok {
			metadata[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	name, description := metadata[frontmatterNameKey], metadata[frontmatterDescriptionKey]
	if name == "" || description == "" {
		return nil, ErrIncompleteAgentPrompt
	}
	body := strings.TrimSpace(text[end+len(frontmatterClose):])
	var out strings.Builder
	fmt.Fprintf(&out, codexNameFormat, strconv.Quote(name), strconv.Quote(description))
	if sandbox := codexSandbox(name); sandbox != "" {
		fmt.Fprintf(&out, codexSandboxFormat, strconv.Quote(sandbox))
	}
	out.WriteString(codexInstructionsOpen)
	out.WriteString(strings.ReplaceAll(body, tomlTripleQuote, tomlEscapedTripleQuote))
	out.WriteString(codexInstructionsClose)
	return []byte(out.String()), nil
}

func codexSandbox(name string) string {
	switch {
	case readOnlyAgents[name]:
		return sandboxReadOnly
	case workspaceWriteAgents[name]:
		return sandboxWorkspaceWrite
	default:
		return ""
	}
}
