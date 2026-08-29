package main

// Trust policy gate — deliberately separate from SPEC conformance.
//
// Nothing in this file is an OKF v0.2 conformance rule. The spec derives a trust
// tier from `verified` (§5.3) and never forbids an actor from appearing in both
// `generated` and `verified`; a bundle that trips this gate is still conformant.
// This is *our* enterprise trust policy: a concept must not carry a verification
// event issued by the same actor that produced its current content, because
// SPEC §5.2 keeps the two roles distinct ("who wrote a concept need not be who
// confirmed it") and a self-issued verification raises the derived trust tier
// with no independent confirmation behind it.
//
// Consequently:
//   - findings NEVER enter okf.LintReport.Conformance,
//   - the gate is opt-in (`-policy-no-self-sign`), so default lint semantics and
//     exit codes are unchanged,
//   - okf-go is not modified.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xSAVIKx/okf-skills/okf-go"
)

// RulePolicySelfSign is the machine-readable id for the self-signed verification
// policy rule. It is namespaced `policy-` so it can never be confused with the
// `okf.Rule*` spec-conformance ids.
const RulePolicySelfSign = "policy-self-signed-verification"

// PolicyFinding is one trust-policy violation. It mirrors okf.Finding's shape but
// is a distinct type so a policy finding can never be appended to the conformance
// list by accident.
type PolicyFinding struct {
	Rule   string `json:"rule"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

// selfSignedActors returns the actors that both produced the current content and
// issued a verification event for it, sorted for determinism. Empty when the
// concept carries no `generated.by`, or no `verified` entry by that actor.
func selfSignedActors(fm okf.Frontmatter) []string {
	if fm.Generated == nil || strings.TrimSpace(fm.Generated.By) == "" {
		return nil
	}
	producer := strings.TrimSpace(fm.Generated.By)
	seen := map[string]bool{}
	var out []string
	for _, v := range fm.Verified {
		if strings.TrimSpace(v.By) == producer && !seen[producer] {
			seen[producer] = true
			out = append(out, producer)
		}
	}
	sort.Strings(out)
	return out
}

// ScanPolicy walks a bundle and reports trust-policy violations. It reads the
// same concept files okf.ScanBundle does but keeps its findings in a separate
// list; reserved files (index.md, log.md) carry no verification state and are
// skipped. An unreadable concept is left to the conformance layer to report.
func ScanPolicy(dir string) ([]PolicyFinding, error) {
	var findings []PolicyFinding
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		switch strings.ToLower(info.Name()) {
		case "index.md", "log.md":
			return nil
		}
		doc, readErr := okf.ReadConceptDoc(p)
		if readErr != nil || doc == nil {
			return nil // malformed concepts are a conformance concern, not a policy one
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)
		for _, actor := range selfSignedActors(doc.Frontmatter) {
			findings = append(findings, PolicyFinding{
				Rule: RulePolicySelfSign,
				Path: rel,
				Detail: fmt.Sprintf(
					"`%s` appears in both `generated.by` and `verified`; a verification must come from an actor other than the one that produced the content",
					actor),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Detail < findings[j].Detail
	})
	return findings, nil
}

// policyReport renders the policy findings under a heading that keeps them
// visibly distinct from spec conformance.
func policyReport(findings []PolicyFinding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- trust policy violations (not spec conformance): %d\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(&b, "    [%s] %s: %s\n", f.Rule, f.Path, f.Detail)
	}
	return b.String()
}
