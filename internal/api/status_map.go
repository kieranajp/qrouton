package api

import (
	"time"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func each[T, M any](items []T, mapped func(T) M) []M {
	out := make([]M, len(items))
	for i, item := range items {
		out[i] = mapped(item)
	}
	return out
}

func chromeFields(f status.Fields) *qroutonv1.ChromeFields {
	return &qroutonv1.ChromeFields{
		BugReportId:         f.BugReportID,
		BugReportStatus:     f.BugReportStatus,
		BugReportLabel:      f.BugReportLabel,
		Mode:                f.Mode,
		Identity:            f.Identity,
		Branch:              f.Branch,
		Slug:                f.Slug,
		Terminal:            f.Terminal,
		Sessions:            each(f.Sessions, sessionRow),
		Documents:           each(f.Documents, sessionDocument),
		RepositoryDocuments: each(f.RepositoryDocuments, repositoryDocuments),
		Repos:               each(f.Repos, repoStat),
		Activity:            f.Activity,
		Stages:              stages(f.Stages),
		StagesLive:          stages(f.StagesLive),
		Root:                f.Root,
		Picker:              f.Picker,
		Welcoming:           f.Welcoming,
		Placing:             f.Placing,
		Stickers:            each(f.Stickers, sticker),
		SessionSummary:      &qroutonv1.SessionSummary{Text: f.SessionSummary.Text, Stage: f.SessionSummary.Stage, At: f.SessionSummary.At},
		PullRequests:        &qroutonv1.PullRequestList{Items: each(f.PullRequests.Items, pullRequest), SignedOut: f.PullRequests.SignedOut},
		SessionDigest:       &qroutonv1.SessionDigest{Text: f.SessionDigest.Text, At: f.SessionDigest.At},
	}
}

func pullRequest(p status.PullRequest) *qroutonv1.GitHubPullRequest {
	return &qroutonv1.GitHubPullRequest{Repo: p.Repo, Number: int32(p.Number), Title: p.Title, Url: p.URL, State: p.State}
}

func sessionRow(r status.SessionRow) *qroutonv1.SessionRow {
	return &qroutonv1.SessionRow{
		Name:     r.Name,
		Slug:     r.Slug,
		Initials: r.Initials,
		Mode:     r.Mode,
		Repos: each(r.Repos, func(repo status.SessionRepo) *qroutonv1.SessionRepo {
			return &qroutonv1.SessionRepo{Name: repo.Name, Role: repo.Role}
		}),
		Terminal: r.Terminal,
		Activity: r.Activity,
		Summary:  agentSummary(r.Summary),
		Unseen:   int32(r.Unseen),
		Opened:   timestamp(r.Opened),
		Sticker:  r.Sticker,
	}
}

func agentSummary(s status.AgentSummary) *qroutonv1.AgentSummary {
	return &qroutonv1.AgentSummary{
		Attention: s.Attention,
		Active:    int32(s.Active),
		Coverage:  s.Coverage,
		Running:   s.Running,
		Tools:     s.Tools,
		State:     s.State,
		Reason:    s.Reason,
		Tool:      s.Tool,
		Stage:     s.Stage,
	}
}

func sticker(s status.Sticker) *qroutonv1.Sticker {
	return &qroutonv1.Sticker{Id: s.ID, Colour: s.Colour, Symbol: s.Symbol, Label: s.Label}
}

func stages(s status.Stages) *qroutonv1.Stages {
	return &qroutonv1.Stages{Research: s.Research, Plan: s.Plan, Implement: s.Implement}
}

func sessionDocument(d status.Document) *qroutonv1.SessionDocument {
	return &qroutonv1.SessionDocument{Name: d.Name, Path: d.Path, Kind: d.Kind, Id: d.ID, At: timestamp(d.At)}
}

func repositoryDocuments(r status.RepositoryDocuments) *qroutonv1.RepositoryDocuments {
	return &qroutonv1.RepositoryDocuments{Name: r.Name, Documents: each(r.Documents, sessionDocument)}
}

func repoStat(r status.RepoStat) *qroutonv1.RepoStat {
	return &qroutonv1.RepoStat{
		Name:       r.Name,
		Role:       r.Role,
		Path:       r.Path,
		Commits:    int32(r.Commits),
		Insertions: int32(r.Insertions),
		Deletions:  int32(r.Deletions),
		Measured:   r.Measured,
	}
}

func agentTree(t status.AgentTree) *qroutonv1.AgentTree {
	return &qroutonv1.AgentTree{
		Provider:      t.Provider,
		ProviderLabel: t.ProviderLabel,
		Coverage:      t.Coverage,
		Tools:         t.Tools,
		Nodes:         each(t.Nodes, agentNode),
	}
}

func agentNode(n status.AgentNode) *qroutonv1.AgentNode {
	return &qroutonv1.AgentNode{
		Id:          n.ID,
		Run:         n.Run,
		Parent:      n.Parent,
		Root:        n.Root,
		Placed:      n.Placed,
		Type:        n.Type,
		Stage:       n.Stage,
		Description: n.Description,
		TaskName:    n.TaskName,
		State:       n.State,
		Reason:      n.Reason,
		Tool:        n.Tool,
		Target:      n.Target,
		Started:     timestamp(n.Started),
		Finished:    timestamp(n.Finished),
		ToolCount:   int32(n.ToolCount),
		Tokens:      int32(n.Tokens),
		LastMessage: n.LastMessage,
		Files:       n.Files,
		Documents:   each(n.Documents, sessionDocument),
		Tools:       each(n.Tools, agentToolEvent),
	}
}

func agentToolEvent(e status.AgentToolEvent) *qroutonv1.AgentToolEvent {
	return &qroutonv1.AgentToolEvent{
		Tool:      e.Tool,
		Target:    e.Target,
		Start:     timestamp(e.Start),
		End:       timestamp(e.End),
		Failed:    e.Failed,
		Abandoned: e.Abandoned,
	}
}
