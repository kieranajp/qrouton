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

// agentPrompt is an agent source split into the parts a runner that rebuilds
// the document, rather than reading the source verbatim, needs.
type agentPrompt struct {
	name        string
	description string
	body        string
}

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
		agent, err := parseAgentPrompt(prompt.Content)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompt.ID, err)
		}
		return []Rendered{
			{Path: claudeAgentsDir + name + promptFileExt, Content: prompt.Content},
			{Path: codexAgentsDir + name + tomlExtension, Content: renderCodexAgent(agent)},
			{Path: agyAgentsDir + name + "/" + agyAgentFileName, Content: renderAgyAgent(agent)},
		}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedPrompt, prompt.ID)
	}
}

func parseAgentPrompt(content []byte) (agentPrompt, error) {
	text := string(content)
	if !strings.HasPrefix(text, frontmatterFence) {
		return agentPrompt{}, ErrNoFrontmatter
	}
	end := strings.Index(text[len(frontmatterFence):], frontmatterClose)
	if end < 0 {
		return agentPrompt{}, ErrUnterminatedFrontmatter
	}
	end += len(frontmatterFence)
	metadata := make(map[string]string)
	for _, line := range strings.Split(text[len(frontmatterFence):end], "\n") {
		key, value, ok := strings.Cut(line, frontmatterKeySep)
		if ok {
			metadata[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	agent := agentPrompt{
		name:        metadata[frontmatterNameKey],
		description: metadata[frontmatterDescriptionKey],
		body:        strings.TrimSpace(text[end+len(frontmatterClose):]),
	}
	if agent.name == "" || agent.description == "" {
		return agentPrompt{}, ErrIncompleteAgentPrompt
	}
	return agent, nil
}

func renderCodexAgent(agent agentPrompt) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, codexNameFormat, strconv.Quote(agent.name), strconv.Quote(agent.description))
	if sandbox := codexSandbox(agent.name); sandbox != "" {
		fmt.Fprintf(&out, codexSandboxFormat, strconv.Quote(sandbox))
	}
	out.WriteString(codexInstructionsOpen)
	out.WriteString(strings.ReplaceAll(agent.body, tomlTripleQuote, tomlEscapedTripleQuote))
	out.WriteString(codexInstructionsClose)
	return []byte(out.String())
}

// renderAgyAgent writes the frontmatter keys agy's own agent files carry, then
// the source body under the single H1 that introduces a system prompt. The
// claude source is not reused verbatim because its frontmatter holds keys agy
// does not read.
func renderAgyAgent(agent agentPrompt) []byte {
	var out strings.Builder
	out.WriteString(frontmatterFence)
	fmt.Fprintf(&out, agyNameFormat, strconv.Quote(agent.name), strconv.Quote(agent.description))
	out.WriteString(agySubagentLine)
	out.WriteString(frontmatterFence)
	fmt.Fprintf(&out, agyPromptHeadingFormat, agent.name)
	out.WriteString(agent.body)
	out.WriteString("\n")
	return []byte(out.String())
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
