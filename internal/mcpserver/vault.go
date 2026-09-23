package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type searchVaultInput struct {
	Query string   `json:"query" jsonschema:"What to look for, in plain words"`
	Kinds []string `json:"kinds,omitempty" jsonschema:"Only these artifact kinds: research, spec, plan, note or dead_end"`
	Limit int      `json:"limit,omitempty" jsonschema:"Documents to take in fused order; defaults to 5"`
}

type readVaultInput struct {
	Ref     string `json:"ref" jsonschema:"A search_thoughts hit's ref, or its artifact id"`
	Line    int    `json:"line,omitempty" jsonschema:"First line to read; omit for the whole document"`
	Through int    `json:"through,omitempty" jsonschema:"Last line to read; defaults to the end of the document"`
}

type openVaultInput struct {
	Ref        string `json:"ref" jsonschema:"A search_thoughts hit's ref, or its artifact id"`
	Line       int    `json:"line,omitempty" jsonschema:"One-based line to draw the user's eye to"`
	Through    int    `json:"through,omitempty" jsonschema:"Last line of the marked range; defaults to line alone"`
	Foreground *bool  `json:"foreground,omitempty" jsonschema:"Sparse logical-selection override: true selects this tab, false keeps it in the background, and omitted uses the tool default"`
}

func addVaultTools(server *mcp.Server, windows *windowManager, host workbench.VaultHost) {
	mcp.AddTool(server, &mcp.Tool{Name: toolSearchThoughts, Description: descSearchThoughts},
		func(ctx context.Context, _ *mcp.CallToolRequest, input searchVaultInput) (*mcp.CallToolResult, any, error) {
			res, err := host.SearchVault(ctx, vault.Query{Text: input.Query, Kinds: input.Kinds, Limit: input.Limit})
			if err != nil {
				return nil, nil, err
			}
			return textResult(formatHits(res)), res, nil
		})
	mcp.AddTool(server, &mcp.Tool{Name: toolReadThoughts, Description: descReadThoughts},
		func(ctx context.Context, _ *mcp.CallToolRequest, input readVaultInput) (*mcp.CallToolResult, any, error) {
			ex, err := host.ReadVault(ctx, vault.ReadRequest{Ref: input.Ref, Line: input.Line, Through: input.Through})
			if err != nil {
				return nil, nil, err
			}
			text := fmt.Sprintf(vaultReadFormat, ex.Title, ex.ID, ex.Ref, ex.Line, ex.Through, ex.Lines) + ex.Content
			if ex.Truncated {
				text += vaultTruncated
			}
			return textResult(text), ex, nil
		})
	addTool(server, toolOpenThoughts, descOpenThoughts, keyMessage, messageOnly(func(ctx context.Context, input openVaultInput) (string, error) {
		ex, err := host.ReadVault(ctx, vault.ReadRequest{Ref: input.Ref, Full: true})
		if err != nil {
			return "", err
		}
		span := workbench.LineSpan{Line: input.Line, Through: input.Through}
		if _, err := windows.open(ctx, vaultWindowName, workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: ex.Title, Content: ex.Content,
			Format: workbench.FormatMarkdown, Span: span, Select: resolveForeground(input.Foreground, true),
		}); err != nil {
			return "", fmt.Errorf("open thoughts document: %w", err)
		}
		if first, last, ok := span.Bounds(); ok {
			return fmt.Sprintf(vaultOpenedSpanFormat, ex.Title, ex.ID, lineRange(first, last)), nil
		}
		return fmt.Sprintf(vaultOpenedFormat, ex.Title, ex.ID), nil
	}))
}

func formatHits(res vault.Result) string {
	var b strings.Builder
	switch res.Status.Semantic {
	case vault.StatusUnavailable:
		fmt.Fprintf(&b, vaultBM25OnlyFormat, res.Status.Lexical, res.Status.Reason)
	case vault.StatusPartial:
		fmt.Fprintf(&b, vaultPartialFormat, res.Status.Pending)
	default:
		b.WriteString(vaultReadyHeader)
	}
	b.WriteString(vaultEvidenceNote)
	if len(res.Hits) == 0 {
		b.WriteString(vaultNoHits)
		return b.String()
	}
	for i, h := range res.Hits {
		kind := h.Kind
		if kind == "" {
			kind = vaultUnknownKind
		}
		fmt.Fprintf(&b, vaultHitFormat, i+1, h.Title, kind, h.ID, h.Ref, lineRange(h.Line, h.Through), h.Breadcrumb)
		var evidence []string
		if h.BM25 != nil {
			evidence = append(evidence, fmt.Sprintf(vaultBM25Format, h.BM25.Rank, h.BM25.Score))
		}
		if h.Dense != nil {
			evidence = append(evidence, fmt.Sprintf(vaultDenseFormat, h.Dense.Rank, h.Dense.Cosine))
		}
		fmt.Fprintf(&b, vaultEvidenceFormat, strings.Join(evidence, vaultEvidenceJoiner), h.FusedRank, h.AdmittedBy, h.Snippet)
	}
	return b.String()
}
