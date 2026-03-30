// Package report — writer.go
package report

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k0-agent/k0/internal/types"
)

// Provisional is a structured provisional report.
type Provisional struct {
	GoalID      string
	Goal        string
	Summary     string
	Findings    []types.Finding
	NextSteps   []string
	RawOutput   map[string]string
	GeneratedAt time.Time
	Markdown    string
}

// LLMClient is the minimal interface report needs from the LLM.
type LLMClient interface {
	Complete(ctx context.Context, system, prompt string) (string, error)
	SuggestNextSteps(ctx context.Context, goal string, findingsSummary string) ([]string, error)
}

// Generate builds a Provisional report from subagent results.
func Generate(ctx context.Context, llm LLMClient, goal string, results []types.SubagentResult) (*Provisional, error) {
	var allFindings []types.Finding
	rawOutputs := make(map[string]string)
	for _, r := range results {
		allFindings = append(allFindings, r.Findings...)
		if r.Output != "" {
			max := len(r.Output)
			if max > 2000 {
				max = 2000
			}
			rawOutputs[r.Label] = r.Output[:max]
		}
	}

	prov := &Provisional{
		Goal:        goal,
		GeneratedAt: time.Now(),
		Findings:    allFindings,
		RawOutput:   rawOutputs,
		Summary:     fmt.Sprintf("%d finding(s) across %d agent(s)", len(allFindings), len(results)),
	}

	// Ask LLM for next steps if we have findings
	if len(allFindings) > 0 {
		summary := buildFindingsSummary(allFindings)
		steps, err := llm.SuggestNextSteps(ctx, goal, summary)
		if err == nil && len(steps) > 0 {
			prov.NextSteps = steps
		} else {
			prov.NextSteps = defaultNextSteps(allFindings)
		}
	} else {
		prov.NextSteps = []string{
			"Review raw tool output for missed indicators.",
			"Consider a deeper scan with different flags or tools.",
			"Expand scope to include additional ports or services.",
		}
	}

	prov.Markdown = renderMarkdown(prov)
	return prov, nil
}

func buildFindingsSummary(findings []types.Finding) string {
	var sb strings.Builder
	for _, f := range findings {
		sb.WriteString(fmt.Sprintf("[%s] %s — %s\n", f.Severity, f.Title, f.Target))
	}
	return sb.String()
}

func defaultNextSteps(findings []types.Finding) []string {
	steps := []string{"Review findings and prioritise by severity."}
	for _, f := range findings {
		if f.Severity == types.SeverityCritical || f.Severity == types.SeverityHigh {
			steps = append(steps, fmt.Sprintf("Investigate %s: %s", f.Target, f.Title))
		}
	}
	return steps
}

func renderMarkdown(p *Provisional) string {
	var sb strings.Builder
	sb.WriteString("# 📄 Provisional Report\n\n")
	sb.WriteString(fmt.Sprintf("**Goal**: %s  \n", p.Goal))
	sb.WriteString(fmt.Sprintf("**Generated**: %s  \n", p.GeneratedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Status**: PROVISIONAL — requires analyst review before delivery  \n\n"))
	sb.WriteString("---\n\n")

	// Severity summary table
	sevCounts := map[types.Severity]int{}
	for _, f := range p.Findings {
		sevCounts[f.Severity]++
	}
	sb.WriteString("## Risk Summary\n\n")
	sb.WriteString("| Severity | Count |\n|---|---|\n")
	for _, sev := range []types.Severity{types.SeverityCritical, types.SeverityHigh, types.SeverityMedium, types.SeverityLow, types.SeverityInfo} {
		if c := sevCounts[sev]; c > 0 {
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", sev, c))
		}
	}
	sb.WriteString("\n")

	// Findings detail
	if len(p.Findings) == 0 {
		sb.WriteString("*No findings extracted. Raw output saved to memory/episodes.*\n\n")
	} else {
		sb.WriteString("## ⚠️ Findings\n\n")
		sb.WriteString("| Severity | Title | Target | CVE |\n|---|---|---|---|\n")
		for _, f := range p.Findings {
			cve := f.CVE
			if cve == "" {
				cve = "—"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", f.Severity, f.Title, f.Target, cve))
		}
		sb.WriteString("\n")

		// Detailed findings
		sb.WriteString("### Finding Details\n\n")
		for i, f := range p.Findings {
			sb.WriteString(fmt.Sprintf("#### %d. [%s] %s\n\n", i+1, f.Severity, f.Title))
			sb.WriteString(fmt.Sprintf("**Target**: %s  \n", f.Target))
			sb.WriteString(fmt.Sprintf("**Description**: %s  \n", f.Description))
			if f.Evidence != "" {
				sb.WriteString(fmt.Sprintf("**Evidence**:\n```\n%s\n```\n", f.Evidence))
			}
			if f.CVE != "" {
				sb.WriteString(fmt.Sprintf("**CVE**: %s  \n", f.CVE))
			}
			sb.WriteString("\n")
		}
	}

	// Raw output snippets
	if len(p.RawOutput) > 0 {
		sb.WriteString("## 📋 Raw Tool Output\n\n")
		for label, out := range p.RawOutput {
			snippet := out
			if len(snippet) > 500 {
				snippet = snippet[:500] + "\n...[truncated]"
			}
			sb.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", label, snippet))
		}
	}

	// Next steps
	if len(p.NextSteps) > 0 {
		sb.WriteString("## 📝 Next Steps\n\n")
		for i, step := range p.NextSteps {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
	}

	sb.WriteString("\n---\n")
	sb.WriteString("*Report saved: ~/.kiai/memory/reports/*\n")
	return sb.String()
}
