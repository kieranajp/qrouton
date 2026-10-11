package launch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/kieranajp/qrouton/prompts"
)

type ReviewInput struct {
	Plan         string            `json:"plan"`
	Spec         string            `json:"spec,omitempty"`
	Bases        map[string]string `json:"bases,omitempty"`
	Brief        string            `json:"brief"`
	Verification string            `json:"verification"`
}

type ReviewerState struct {
	Mode      string           `json:"mode"`
	Enabled   bool             `json:"enabled"`
	Available bool             `json:"available"`
	Settings  *config.Reviewer `json:"settings,omitempty"`
	Error     string           `json:"error,omitempty"`
}

type PreparedReview struct {
	RunID       string                  `json:"run_id"`
	WindowName  string                  `json:"window_name"`
	ReportPath  string                  `json:"report_path"`
	RequestPath string                  `json:"request_path"`
	Settings    config.Reviewer         `json:"settings"`
	Window      workbench.WindowOptions `json:"-"`
}

type reviewRepository struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Base string `json:"base"`
	Head string `json:"head"`
}

type reviewRequest struct {
	RunID        string             `json:"run_id"`
	Root         string             `json:"root"`
	Bin          string             `json:"bin"`
	Plan         string             `json:"plan"`
	Spec         string             `json:"spec,omitempty"`
	Report       string             `json:"report"`
	Settings     config.Reviewer    `json:"settings"`
	Command      []string           `json:"command"`
	Override     bool               `json:"override"`
	Handle       workbench.Handle   `json:"handle"`
	Editor       EditorCommand      `json:"editor"`
	Repositories []reviewRepository `json:"repositories"`
	Role         string             `json:"role"`
	Task         string             `json:"task"`
}

func ReviewerSettings(cfg *config.Config) ReviewerState {
	snapshot := cfg.Snapshot()
	if snapshot.Reviewer == nil {
		return ReviewerState{Mode: ReviewModeStandard}
	}
	state := ReviewerState{Mode: ReviewModeAdversarial, Enabled: true, Settings: snapshot.Reviewer}
	if err := ValidateReviewer(snapshot.Reviewer, snapshot.Launch); err != nil {
		state.Error = err.Error()
	} else {
		state.Available = true
	}
	return state
}

func PrepareReview(ctx context.Context, cfg *config.Config, root, bin string, handle workbench.Handle, editor EditorCommand, input ReviewInput) (PreparedReview, error) {
	snapshot := cfg.Snapshot()
	if snapshot.Reviewer == nil {
		return PreparedReview{}, ErrReviewerDisabled
	}
	if err := ctx.Err(); err != nil {
		return PreparedReview{}, err
	}
	if err := ValidateReviewer(snapshot.Reviewer, snapshot.Launch); err != nil {
		return PreparedReview{}, err
	}
	for _, value := range []string{input.Plan, input.Spec, input.Brief, input.Verification} {
		if len(value) > reviewFieldLimit || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			return PreparedReview{}, ErrReviewRequest
		}
	}
	if strings.TrimSpace(input.Plan) == "" || strings.TrimSpace(input.Brief) == "" {
		return PreparedReview{}, ErrReviewRequest
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return PreparedReview{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return PreparedReview{}, err
	}
	if handle.Socket == "" || bin == "" {
		return PreparedReview{}, ErrReviewRequest
	}
	handleRoot, err := filepath.EvalSymlinks(handle.SessionRoot)
	if err != nil || handleRoot != root {
		return PreparedReview{}, ErrReviewRequest
	}
	plan, err := reviewArtifact(root, input.Plan)
	if err != nil {
		return PreparedReview{}, err
	}
	spec := ""
	if input.Spec != "" {
		spec, err = reviewArtifact(root, input.Spec)
		if err != nil {
			return PreparedReview{}, err
		}
	}
	repositories, err := pinReviewRepositories(ctx, root, input.Bases)
	if err != nil {
		return PreparedReview{}, err
	}
	role, err := prompts.ImplementationReviewer(ctx)
	if err != nil {
		return PreparedReview{}, err
	}
	reportDir, err := reviewReportDirectory(root)
	if err != nil {
		return PreparedReview{}, err
	}
	planLink, err := filepath.Rel(reportDir, plan)
	if err != nil {
		return PreparedReview{}, fmt.Errorf("%w: %w", ErrReviewRequest, err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(reportDir, planLink))
	if err != nil || resolved != plan {
		return PreparedReview{}, fmt.Errorf("%w: %s", ErrReviewRequest, plan)
	}
	runners, err := Runners(snapshot)
	if err != nil {
		return PreparedReview{}, err
	}
	selected := slices.IndexFunc(runners, func(r Runner) bool { return r.ID == strings.TrimSpace(snapshot.Reviewer.Harness) })
	if selected < 0 || runners[selected].Path == "" {
		return PreparedReview{}, ErrRunnerUnavailable
	}
	runner := runners[selected]
	specification, _ := specFor(runner.ID)
	command := reviewCommand(runner.Command, specification.ReviewOverrides, root)
	settings := *snapshot.Reviewer
	settings.Harness, settings.Model, settings.Reasoning = strings.TrimSpace(settings.Harness), strings.TrimSpace(settings.Model), strings.TrimSpace(settings.Reasoning)
	if err := ctx.Err(); err != nil {
		return PreparedReview{}, err
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return PreparedReview{}, err
	}
	id := hex.EncodeToString(idBytes)
	runDir := sessionpaths.ReviewRun(root, id)
	if err := os.MkdirAll(sessionpaths.ReviewRuns(root), reviewPrivateMode); err != nil {
		return PreparedReview{}, fmt.Errorf("%w: %w", ErrReviewAllocation, err)
	}
	if err := os.Mkdir(runDir, reviewPrivateMode); err != nil {
		return PreparedReview{}, fmt.Errorf("%w: %w", ErrReviewAllocation, err)
	}
	complete := false
	report := sessionpaths.ReviewReport(root, id)
	reportCreated := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(runDir)
			if reportCreated {
				_ = os.Remove(report)
			}
		}
	}()
	if err := os.MkdirAll(reportDir, dirMode); err != nil {
		return PreparedReview{}, fmt.Errorf("%w: %w", ErrReviewAllocation, err)
	}
	scope := reviewScopeText(repositories)
	reasoning := settings.Reasoning
	if reasoning == "" {
		reasoning = reviewDefaultReasoning
	}
	title := reviewTitlePrefix + id
	placeholder := fmt.Sprintf(reviewPlaceholderFormat, quotedConfigString(title), quotedConfigString(filepath.ToSlash(planLink)), title, filepath.ToSlash(planLink), settings.Harness, settings.Model, reasoning, scope)
	if err := writeReviewFile(report, []byte(placeholder), reviewReportMode); err != nil {
		return PreparedReview{}, fmt.Errorf("%w: %w", ErrReviewAllocation, err)
	}
	reportCreated = true
	request := reviewRequest{RunID: id, Root: root, Bin: bin, Plan: plan, Spec: spec, Report: report, Settings: settings, Command: command, Override: runner.Override, Handle: handle, Editor: editor, Repositories: repositories, Role: role,
		Task: fmt.Sprintf(reviewTaskFormat, root, plan, spec, report, settings.Harness, settings.Model, reasoning, scope, input.Brief, input.Verification)}
	raw, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return PreparedReview{}, err
	}
	requestPath := sessionpaths.ReviewRequest(root, id)
	if err := writeReviewFile(requestPath, raw, reviewRequestMode); err != nil {
		return PreparedReview{}, fmt.Errorf("%w: %w", ErrReviewAllocation, err)
	}
	if err := ctx.Err(); err != nil {
		return PreparedReview{}, err
	}
	complete = true
	name := reviewWindowPrefix + id
	return PreparedReview{RunID: id, WindowName: name, ReportPath: report, RequestPath: requestPath, Settings: settings, Window: workbench.WindowOptions{Kind: workbench.KindTerminal, Label: name, Cwd: specification.ReviewCwd(root, id), Command: []string{bin, ReviewerSubcommand, flagPrefix + ReviewRequestFlag, requestPath}, CloseOnExit: false, Select: false}}, nil
}

func reviewCommand(command []string, policy reviewerOverridePolicy, root string) []string {
	command = slices.Clone(command)
	resolve := func(value string) string {
		if value == "" || filepath.IsAbs(value) || strings.HasPrefix(strings.TrimSpace(value), "{") {
			return value
		}
		return filepath.Join(root, value)
	}
	if strings.ContainsRune(command[0], filepath.Separator) {
		command[0] = resolve(command[0])
	}
	for i := 1; i < len(command); i++ {
		flag, value, inline := strings.Cut(command[i], "=")
		if !slices.Contains(policy.PathFlags, flag) && !slices.Contains(policy.PathListFlags, flag) {
			continue
		}
		if inline {
			command[i] = flag + "=" + resolve(value)
		} else if i+1 < len(command) {
			i++
			command[i] = resolve(command[i])
		}
		if slices.Contains(policy.PathListFlags, flag) {
			for i+1 < len(command) && !strings.HasPrefix(command[i+1], "-") {
				i++
				command[i] = resolve(command[i])
			}
		}
	}
	return command
}

func reviewArtifact(root, path string) (string, error) {
	resolved, err := ResolveSessionFile(root, path)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrReviewRequest, err)
	}
	if strings.ToLower(filepath.Ext(resolved)) != ".md" {
		return "", fmt.Errorf("%w: %s", ErrReviewRequest, path)
	}
	return resolved, nil
}

func reviewReportDirectory(root string) (string, error) {
	shared, err := ResolveSessionDir(root, sessionpaths.Thoughts(root))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrReviewRequest, err)
	}
	reports := sessionpaths.Reviews(root)
	if _, err := os.Lstat(reports); os.IsNotExist(err) {
		return filepath.Join(shared, filepath.Base(reports)), nil
	}
	return ResolveSessionDir(root, reports)
}

func writeReviewFile(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func pinReviewRepositories(ctx context.Context, root string, bases map[string]string) ([]reviewRepository, error) {
	manifest, err := session.Load(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReviewScope, err)
	}
	seen := map[string]bool{}
	var repositories []reviewRepository
	for _, repo := range manifest.Repos {
		if !repo.Role.IsEditing() {
			continue
		}
		id := reviewRepositoryID(repo)
		if repo.Name == "" || seen[id] {
			return nil, fmt.Errorf("%w: %q", ErrReviewScope, id)
		}
		seen[id] = true
		path, err := ResolveSessionDir(root, repo.WorktreePath)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrReviewScope, repo.Name, err)
		}
		top, err := reviewGit(ctx, path, "rev-parse", "--show-toplevel")
		if err != nil {
			return nil, err
		}
		top, err = filepath.EvalSymlinks(top)
		if err != nil || top != path {
			return nil, fmt.Errorf("%w: %s", ErrReviewScope, path)
		}
		head, err := reviewCommit(ctx, path, "HEAD")
		if err != nil {
			return nil, err
		}
		baseRef, explicit := bases[id]
		if !explicit {
			baseRef = repo.BaseBranch
			if baseRef == "" {
				baseRef = repo.DefaultBranch
			}
		}
		if strings.TrimSpace(baseRef) == "" || len(baseRef) > reviewFieldLimit {
			return nil, fmt.Errorf("%w: %s", ErrReviewScope, repo.Name)
		}
		base, err := reviewCommit(ctx, path, baseRef)
		if err != nil {
			return nil, err
		}
		if !explicit {
			base, err = reviewGit(ctx, path, "merge-base", "--all", head, base)
			if err != nil {
				return nil, err
			}
			if len(strings.Fields(base)) != 1 {
				return nil, fmt.Errorf("%w: %s", ErrReviewScope, repo.Name)
			}
		}
		repositories = append(repositories, reviewRepository{Name: id, Path: path, Base: base, Head: head})
	}
	for name := range bases {
		if !seen[name] {
			return nil, fmt.Errorf("%w: %s", ErrReviewScope, name)
		}
	}
	if len(repositories) == 0 {
		return nil, ErrReviewScope
	}
	return repositories, nil
}

func reviewRepositoryID(repo session.ManifestRepo) string {
	if repo.Org == "" {
		return repo.Name
	}
	return repo.Org + "/" + repo.Name
}

func reviewCommit(ctx context.Context, path, ref string) (string, error) {
	return reviewGit(ctx, path, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
}

func reviewGit(ctx context.Context, path string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...)
	output, err := cmd.CombinedOutput()
	value := strings.TrimSpace(string(output))
	if err != nil || strings.Contains(value, "\n") || value == "" {
		return "", fmt.Errorf("%w: %s: %s: %v", ErrReviewScope, path, value, err)
	}
	return value, nil
}

func reviewScopeText(repositories []reviewRepository) string {
	var out strings.Builder
	for _, repo := range repositories {
		fmt.Fprintf(&out, reviewRepoFormat, repo.Name, repo.Path, repo.Base, repo.Head)
	}
	return out.String()
}

func reviewerLaunch(request reviewRequest) ([]string, []string, error) {
	spec, ok := specFor(request.Settings.Harness)
	if !ok || len(request.Command) == 0 {
		return nil, nil, ErrReviewRequest
	}
	argv := spec.ReviewArgs(slices.Clone(request.Command), request.Settings, request.Role)
	task := request.Task
	if spec.ReviewOpeningRole {
		task = request.Role + noticeSeparator + task
	}
	argv = spec.Prompt(argv, task)
	return spec.Inject(argv, injectContext{qroutonBin: request.Bin, dir: request.Root, handle: request.Handle, override: request.Override, reviewer: true, reviewHome: sessionpaths.ReviewAgyHome(request.Root, request.RunID), reviewRunID: request.RunID, reviewRole: request.Role,
		mcpArgs: []string{MCPSubcommand, sessionRootFlag, request.Root, editorJSONFlag, request.Editor.Marshal(), workbenchJSONFlag, request.Handle.Marshal()}})
}

func RunReviewer(requestPath string) error {
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrReviewRequest, err)
	}
	var request reviewRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return fmt.Errorf("%w: %w", ErrReviewRequest, err)
	}
	if request.Root == "" || request.RunID == "" || request.Role == "" || request.Task == "" || len(request.Command) == 0 {
		return ErrReviewRequest
	}
	argv, env, err := reviewerLaunch(request)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrReviewLaunch, err)
	}
	specification, _ := specFor(request.Settings.Harness)
	if err := os.Chdir(specification.ReviewCwd(request.Root, request.RunID)); err != nil {
		return fmt.Errorf("%w: %w", ErrReviewLaunch, err)
	}
	path, err := findExecutable(request.Command[0])
	if err != nil || path == "" {
		return fmt.Errorf("%w: %w: %s", ErrReviewLaunch, ErrRunnerUnavailable, request.Command[0])
	}
	if err := syscall.Exec(path, argv, env); err != nil {
		return fmt.Errorf("%w: %w", ErrReviewLaunch, err)
	}
	return nil
}
