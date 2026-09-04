package prompts

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// embeddedPromptIDs is every prompt the binary carries, in the order List sorts
// them. Naming them is what makes an addition or a loss legible in the failure.
var embeddedPromptIDs = []string{
	"agents/code-reviewer",
	"agents/codebase-researcher",
	"agents/external-researcher",
	"agents/pattern-finder",
	"agents/qrouton-implementation-lead",
	"agents/qrouton-planning-lead",
	"agents/qrouton-research-lead",
	"agents/qrouton-researcher",
	"agents/test-verifier",
	"agents/thoughts-researcher",
	"assistant",
	"orchestrator",
	"skills/qrouton-development",
	"skills/qrouton-evals",
	"skills/qrouton-review",
	"skills/qrouton-slides",
	"skills/qrspi-implement",
	"skills/qrspi-plan",
	"skills/qrspi-questions",
	"skills/qrspi-research",
	"skills/qrspi-spec",
}

func TestQroutonSkillsAreNarrowSoloEntrypoints(t *testing.T) {
	cases := []struct {
		name              string
		descriptionClaims []string
		bodyClaims        []string
	}{
		{
			name:              "qrouton-development",
			descriptionClaims: []string{"Implement or debug qrouton's", "Do not use for operating qrouton on an unrelated project"},
			bodyClaims:        []string{"nearest `AGENTS.md`", "src/lib/bridge/generated.js", "`prompts/`", "npm run test:unit", "npm run test:browser", "GOCACHE=/tmp/qrouton-go-cache make check", "do not infer that a release path runs"},
		},
		{
			name:              "qrouton-evals",
			descriptionClaims: []string{"eval/", "internal/evalharness/", "cmd/qrouton-eval/", "Do not use for ordinary application or unit-test work"},
			bodyClaims:        []string{"`eval/README.md`", "`prompts.Stamp`", "`session.Manifest`", "--no-judge", "QROUTON_EVAL_SMOKE=1", "opt-in and do not run in CI"},
		},
		{
			name:              "qrouton-review",
			descriptionClaims: []string{"pre-merge diff review", "Do not use to implement"},
			bodyClaims:        []string{"review read-only", "generated-source ownership", "launch/eval parity", "GOCACHE=/tmp/qrouton-go-cache make check", "UI, platform, authenticated eval"},
		},
	}

	loader := NewEmbeddedLoader()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prompt, err := loader.Load(context.Background(), ID(skillIDPrefix+tc.name))
			if err != nil {
				t.Fatal(err)
			}
			if len(prompt.Files) != 0 {
				t.Fatalf("solo skill ships supporting files: %#v", prompt.Files)
			}
			name, description, body := skillParts(t, string(prompt.Content))
			if name != tc.name {
				t.Fatalf("skill name = %q", name)
			}
			if strings.TrimSpace(description) == "" {
				t.Fatal("skill description is empty")
			}
			for _, claim := range tc.descriptionClaims {
				if !strings.Contains(description, claim) {
					t.Errorf("description is missing discovery boundary %q", claim)
				}
			}
			for _, claim := range tc.bodyClaims {
				if !strings.Contains(body, claim) {
					t.Errorf("body is missing verified routing claim %q", claim)
				}
			}
			for _, unsupported := range []string{"CI runs authenticated evals", "release runs make check", "comment checks decide comment quality"} {
				if strings.Contains(strings.ToLower(body), strings.ToLower(unsupported)) {
					t.Errorf("body makes unsupported claim %q", unsupported)
				}
			}
		})
	}
}

// Codex will not follow a symlinked SKILL.md, only a symlinked skill folder, so
// each skill links once at its folder rather than once per file inside it.
func TestQroutonSkillsStampIntoBothDiscoveryTrees(t *testing.T) {
	dir := t.TempDir()
	if err := Stamp(context.Background(), dir, NewEmbeddedLoader(), OrchestratorAsset); err != nil {
		t.Fatal(err)
	}
	var skills []string
	for _, id := range embeddedPromptIDs {
		if name, ok := strings.CutPrefix(id, skillIDPrefix); ok {
			skills = append(skills, name)
		}
	}
	for _, root := range []string{claudeSkillsDir, agentsSkillsDir} {
		for _, name := range skills {
			skillLink := filepath.Join(dir, root, skillsDirName, name)
			info, err := os.Lstat(skillLink)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%s is not a stamped discovery link: %v", skillLink, err)
			}
			content, err := os.ReadFile(filepath.Join(skillLink, skillFileName))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), "name: "+name) || !strings.Contains(string(content), MarkerText) {
				t.Fatalf("%s does not resolve to marked skill content:\n%s", skillLink, content)
			}
		}

		reference := filepath.Join(dir, root, skillsDirName, "qrspi-plan", "references", "plan-shape.md")
		content, err := os.ReadFile(reference)
		if err != nil {
			t.Fatalf("%s does not resolve through the skill folder link: %v", reference, err)
		}
		if !strings.Contains(string(content), "### Verify") {
			t.Fatalf("%s did not resolve to the plan shape reference:\n%s", reference, content)
		}
	}
}

// A session stamped before skills linked at the folder carries a real directory
// of file links where the folder link now belongs. Stamping over it has to
// replace it, since every session that predates the change starts in that shape.
func TestStampReplacesPerFileSkillDirectory(t *testing.T) {
	dir := t.TempDir()
	const name = "qrspi-plan"
	canonicalSkill := filepath.Join(sessionpaths.CanonicalPrompts(dir), skillsDirName, name)

	for _, root := range []string{claudeSkillsDir, agentsSkillsDir} {
		stale := filepath.Join(dir, root, skillsDirName, name)
		if err := os.MkdirAll(filepath.Join(stale, "references"), dirMode); err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{skillFileName, filepath.Join("references", "plan-shape.md")} {
			link := filepath.Join(stale, file)
			relative, err := filepath.Rel(filepath.Dir(link), filepath.Join(canonicalSkill, file))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(relative, link); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := Stamp(context.Background(), dir, NewEmbeddedLoader(), OrchestratorAsset); err != nil {
		t.Fatal(err)
	}

	for _, root := range []string{claudeSkillsDir, agentsSkillsDir} {
		skillLink := filepath.Join(dir, root, skillsDirName, name)
		info, err := os.Lstat(skillLink)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is not a stamped folder link: %v", skillLink, err)
		}
		content, err := os.ReadFile(filepath.Join(skillLink, skillFileName))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "name: "+name) {
			t.Fatalf("%s does not resolve to the skill:\n%s", skillLink, content)
		}
	}
}

// A qrouton old enough to link skills per file removes SKILL.md through the
// folder link, which unlinks the canonical file and leaves a symlink in its
// place. Stamping again has to restore the file rather than write through it.
func TestStampRestoresCanonicalAssetReplacedByALink(t *testing.T) {
	dir := t.TempDir()
	if err := Stamp(context.Background(), dir, NewEmbeddedLoader(), OrchestratorAsset); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(sessionpaths.CanonicalPrompts(dir), skillsDirName, "qrspi-plan", skillFileName)
	if err := os.Remove(canonical); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "..", "gone", skillFileName), canonical); err != nil {
		t.Fatal(err)
	}

	if err := Stamp(context.Background(), dir, NewEmbeddedLoader(), OrchestratorAsset); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(canonical)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("%s is still a link: %v", canonical, err)
	}
	content, err := os.ReadFile(canonical)
	if err != nil || !strings.Contains(string(content), "name: qrspi-plan") {
		t.Fatalf("canonical skill was not restored: %v", err)
	}
}

// Replacing a stale directory must not become a licence to delete a directory
// the user owns, whatever its name.
func TestStampRefusesSkillDirectoryHoldingUserContent(t *testing.T) {
	dir := t.TempDir()
	mine := filepath.Join(dir, claudeSkillsDir, skillsDirName, "qrspi-plan")
	if err := os.MkdirAll(mine, dirMode); err != nil {
		t.Fatal(err)
	}
	authored := filepath.Join(mine, skillFileName)
	if err := os.WriteFile(authored, []byte("my own skill"), fileMode); err != nil {
		t.Fatal(err)
	}

	err := Stamp(context.Background(), dir, NewEmbeddedLoader(), OrchestratorAsset)
	if !errors.Is(err, ErrUserOwnedAsset) {
		t.Fatalf("Stamp over a user-owned skill directory = %v", err)
	}
	content, err := os.ReadFile(authored)
	if err != nil || string(content) != "my own skill" {
		t.Fatalf("the user's own skill did not survive: %q %v", content, err)
	}
}

// A stale per-file skill directory holding our symlinks alongside one file the
// user added is neither of the pure cases stampedTree distinguishes: it must
// still refuse, since only every file being ours makes the directory ours.
func TestStampRefusesSkillDirectoryHoldingMixedContent(t *testing.T) {
	dir := t.TempDir()
	const name = "qrspi-plan"
	canonicalSkill := filepath.Join(sessionpaths.CanonicalPrompts(dir), skillsDirName, name)

	stale := filepath.Join(dir, claudeSkillsDir, skillsDirName, name)
	if err := os.MkdirAll(stale, dirMode); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(stale, skillFileName)
	relative, err := filepath.Rel(filepath.Dir(link), filepath.Join(canonicalSkill, skillFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(relative, link); err != nil {
		t.Fatal(err)
	}
	authored := filepath.Join(stale, "notes.md")
	if err := os.WriteFile(authored, []byte("my own notes"), fileMode); err != nil {
		t.Fatal(err)
	}

	err = Stamp(context.Background(), dir, NewEmbeddedLoader(), OrchestratorAsset)
	if !errors.Is(err, ErrUserOwnedAsset) {
		t.Fatalf("Stamp over a mixed skill directory = %v", err)
	}
	content, err := os.ReadFile(authored)
	if err != nil || string(content) != "my own notes" {
		t.Fatalf("the user's own file did not survive: %q %v", content, err)
	}
}

func skillParts(t *testing.T, content string) (string, string, string) {
	t.Helper()
	if !strings.HasPrefix(content, frontmatterFence) {
		t.Fatal("skill has no frontmatter")
	}
	end := strings.Index(content[len(frontmatterFence):], frontmatterClose)
	if end < 0 {
		t.Fatal("skill frontmatter is unterminated")
	}
	end += len(frontmatterFence)
	fields := map[string]string{}
	for _, line := range strings.Split(content[len(frontmatterFence):end], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return fields["name"], fields["description"], content[end+len(frontmatterClose):]
}

func TestEmbeddedLoaderAndAgentRendering(t *testing.T) {
	loader := NewEmbeddedLoader()
	prompt, err := loader.Load(context.Background(), ID(agentIDPrefix+"qrouton-research-lead"))
	if err != nil {
		t.Fatal(err)
	}
	assets, err := Render(prompt)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 || assets[0].Path != ".claude/agents/qrouton-research-lead.md" || assets[1].Path != ".codex/agents/qrouton-research-lead.toml" {
		t.Fatalf("rendered assets = %#v", assets)
	}
	if !strings.Contains(string(assets[1].Content), `sandbox_mode = "workspace-write"`) || !strings.Contains(string(assets[1].Content), "Lead a bounded") {
		t.Fatalf("Codex rendering missing metadata or instructions:\n%s", assets[1].Content)
	}
	all, err := loader.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(all))
	for i, p := range all {
		ids[i] = string(p.ID)
	}
	if !slices.Equal(ids, embeddedPromptIDs) {
		t.Fatalf("embedded prompts = %#v, want %#v", ids, embeddedPromptIDs)
	}
}

func TestSubagentChoiceExpandedForDelegatingPrompts(t *testing.T) {
	loader := NewEmbeddedLoader()
	ids := []ID{
		Orchestrator,
		Assistant,
		ID(agentIDPrefix + "qrouton-implementation-lead"),
		ID(agentIDPrefix + "qrouton-planning-lead"),
		ID(agentIDPrefix + "qrouton-research-lead"),
	}
	for _, id := range ids {
		prompt, err := loader.Load(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		content := string(prompt.Content)
		if !strings.Contains(content, "Default to the efficient tier (to keep fan-out costs from ballooning)") {
			t.Errorf("prompt %q missing subagent choice guidance", id)
		}
		if strings.Contains(content, subagentChoicePlaceholder) {
			t.Errorf("prompt %q retained subagent choice placeholder", id)
		}
	}
}

// Both modes drive the same workbench, so the description of it is one text.
// What differs is escalation: only the assistant has the tool, and only its
// prompt may say so.
func TestWorkspaceWindowsSharedByBothModePrompts(t *testing.T) {
	loader := NewEmbeddedLoader()
	rendered := map[ID]string{}
	for _, id := range []ID{Orchestrator, Assistant} {
		prompt, err := loader.Load(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		content := string(prompt.Content)
		// Any surviving brace pair is a partial that did not expand — including
		// one a partial itself named, which a single expansion pass cannot reach.
		if i := strings.Index(content, "{{"); i >= 0 {
			t.Errorf("prompt %q ships an unexpanded partial: %.32q", id, content[i:])
		}
		if !strings.Contains(content, "## The workspace windows") {
			t.Errorf("prompt %q does not describe the workspace windows", id)
		}
		rendered[id] = content
	}

	shared, err := fs.ReadFile(embedded, workspaceWindowsFileName)
	if err != nil {
		t.Fatal(err)
	}
	for id, content := range rendered {
		if !strings.Contains(content, string(shared)) {
			t.Errorf("prompt %q carries its own copy of the workspace windows section", id)
		}
		for _, guidance := range []string{
			"begins in the background",
			"`foreground: true`",
			"never takes the keyboard",
			"automatically selects `thoughts/` artifacts",
			"waiting marker",
		} {
			if !strings.Contains(content, guidance) {
				t.Errorf("prompt %q is missing window attention guidance %q", id, guidance)
			}
		}
	}

	if strings.Contains(rendered[Orchestrator], "escalat") {
		t.Error("the orchestrator prompt offers escalation, which is the assistant's alone")
	}
	if !strings.Contains(rendered[Assistant], "`escalate`") {
		t.Error("the assistant prompt does not name the escalate tool")
	}
}

// A skill is a folder: a short entry file and the references it defers detail
// to. Nothing forces a skill that needs only one file to grow a second.
func TestASkillFolderShipsItsReferencesAndASoloSkillStaysSolo(t *testing.T) {
	loader := NewFSLoader(fstest.MapFS{
		"skills/solo/SKILL.md":                 {Data: []byte("---\nname: solo\n---\n\nAll of it, here.\n")},
		"skills/folder/SKILL.md":               {Data: []byte("---\nname: folder\n---\n\nSee references/detail.md.\n")},
		"skills/folder/references/detail.md":   {Data: []byte("# Detail\n")},
		"skills/folder/references/_partial.md": {Data: []byte("# Partial\n")},
		"skills/folder/scripts/run.py":         {Data: []byte("print(\"hi\")\n")},
		"skills/folder/.DS_Store":              {Data: []byte("junk")},
		"skills/folder/.git/config":            {Data: []byte("junk")},
	})
	ctx := context.Background()

	solo, err := loader.Load(ctx, ID("skills/solo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(solo.Files) != 0 {
		t.Fatalf("a single-file skill carries %#v", solo.Files)
	}
	assets, err := Render(solo)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Path != "skills/solo/SKILL.md" {
		t.Fatalf("rendered assets = %#v", assets)
	}

	folder, err := loader.Load(ctx, ID("skills/folder"))
	if err != nil {
		t.Fatal(err)
	}
	assets, err = Render(folder)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, asset := range assets {
		paths = append(paths, asset.Path)
	}
	want := []string{
		"skills/folder/SKILL.md",
		"skills/folder/references/_partial.md",
		"skills/folder/references/detail.md",
		"skills/folder/scripts/run.py",
	}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("rendered paths = %v, want %v", paths, want)
	}

	// The whole folder reaches the runner discovery tree, and only the formats
	// whose comment syntax we know carry the generated-by marker. Nothing hidden
	// goes with it.
	dir := t.TempDir()
	if err := Stamp(ctx, dir, loader, OrchestratorAsset); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{claudeSkillsDir, agentsSkillsDir} {
		skillLink := filepath.Join(dir, root, skillsDirName, "folder")
		info, err := os.Lstat(skillLink)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s is not a symlink: %v", skillLink, err)
		}
		reference := filepath.Join(skillLink, "references", "detail.md")
		content, err := os.ReadFile(reference)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(content), MarkerText) {
			t.Fatalf("%s missing the generated-by marker:\n%s", reference, content)
		}
		script, err := os.ReadFile(filepath.Join(dir, root, skillsDirName, "folder", "scripts", "run.py"))
		if err != nil {
			t.Fatal(err)
		}
		if string(script) != "print(\"hi\")\n" {
			t.Fatalf("a script was marked as if it were markdown:\n%s", script)
		}
		if _, err := os.Stat(filepath.Join(dir, root, skillsDirName, "solo", "references")); !os.IsNotExist(err) {
			t.Fatalf("a single-file skill grew a references directory: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, root, skillsDirName, "folder", ".DS_Store")); !os.IsNotExist(err) {
			t.Fatalf("a hidden file was stamped as part of the skill: %v", err)
		}
	}
}

// The plan template lives in the plan skill's own reference file, so SKILL.md
// stays short enough to skim.
func TestPlanSkillDefersItsTemplateToAReference(t *testing.T) {
	prompt, err := NewEmbeddedLoader().Load(context.Background(), ID(skillIDPrefix+"qrspi-plan"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(prompt.Content), "### Verify") {
		t.Error("SKILL.md still holds the plan template")
	}
	if !strings.Contains(string(prompt.Content), "references/plan-shape.md") {
		t.Error("SKILL.md does not point at its reference")
	}
	var found bool
	for _, file := range prompt.Files {
		if file.Path == "references/plan-shape.md" && strings.Contains(string(file.Content), "### Verify") {
			found = true
		}
	}
	if !found {
		t.Errorf("the plan skill ships %#v, none of them the template", prompt.Files)
	}
}

// The research document's shape is one file, read by the workbench pane and
// written by the lead, so the skill points at it rather than restating it.
func TestResearchSkillDefersItsShapeToAReference(t *testing.T) {
	prompt, err := NewEmbeddedLoader().Load(context.Background(), ID(skillIDPrefix+"qrspi-research"))
	if err != nil {
		t.Fatal(err)
	}
	headings := []string{"## Summary", "## Open Questions"}
	for _, heading := range headings {
		if strings.Contains(string(prompt.Content), heading) {
			t.Errorf("SKILL.md still holds %q from the research template", heading)
		}
	}
	if !strings.Contains(string(prompt.Content), "references/research-shape.md") {
		t.Error("SKILL.md does not point at its reference")
	}
	var found bool
	for _, file := range prompt.Files {
		if file.Path != "references/research-shape.md" {
			continue
		}
		found = true
		for _, heading := range headings {
			if !strings.Contains(string(file.Content), heading) {
				t.Errorf("the shape reference is missing %q", heading)
			}
		}
	}
	if !found {
		t.Errorf("the research skill ships %#v, none of them the shape", prompt.Files)
	}
}

// With whole folders embedded, a SKILL.md deeper inside a skill is one of that
// skill's files rather than a skill the loader lists twice.
func TestOnlyAFolderDirectlyUnderSkillsIsASkill(t *testing.T) {
	loader := NewFSLoader(fstest.MapFS{
		"skills/demo/SKILL.md":            {Data: []byte("---\nname: demo\n---\n")},
		"skills/demo/references/SKILL.md": {Data: []byte("# Not a skill\n")},
	})
	listed, err := loader.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != ID("skills/demo") {
		t.Fatalf("listed %#v", listed)
	}
	if len(listed[0].Files) != 1 || listed[0].Files[0].Path != "references/SKILL.md" {
		t.Fatalf("the nested file is not the skill's own: %#v", listed[0].Files)
	}
}

// The template is the contract between the step that frames a research document
// and the readers that ask whether it has been answered yet, so read it the way
// they do.
func TestTheResearchTemplateReadsAsAFramedDocument(t *testing.T) {
	prompt, err := NewEmbeddedLoader().Load(context.Background(), ID(skillIDPrefix+"qrspi-research"))
	if err != nil {
		t.Fatal(err)
	}
	var shape string
	for _, file := range prompt.Files {
		if file.Path == "references/research-shape.md" {
			shape = string(file.Content)
		}
	}
	template, _, found := strings.Cut(shape, "\n```\n")
	if !found {
		t.Fatal("the shape reference holds no fenced template")
	}
	_, template, found = strings.Cut(template, "```markdown\n")
	if !found {
		t.Fatal("the shape reference's template is not fenced as markdown")
	}

	sections := markdown.Sections(template)
	if len(sections) != 3 {
		t.Fatalf("the template opens %#v", sections)
	}
	if sections[0].Name != "Summary" || sections[0].State != markdown.SectionAnswered {
		t.Errorf("the pinned summary reads as %#v", sections[0])
	}
	if sections[1].State != markdown.SectionFramed {
		t.Errorf("a question reads as %#v rather than framing awaiting an answer", sections[1])
	}
	if sections[2].Name != "Open Questions" || sections[2].State != markdown.SectionEmpty {
		t.Errorf("the closing section reads as %#v", sections[2])
	}
}
