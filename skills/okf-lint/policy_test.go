package main

// Deterministic fixtures for the self-signed-verification trust policy gate.
// No agent is executed; each fixture is a frontmatter state fed straight through
// ScanPolicy. These assert *our policy*, not OKF v0.2 conformance — every bundle
// below is spec-conformant.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	actorA = "process:claude-enrich"
	actorB = "process:finance-nightly"
	human  = "human:vavagirls"
)

// writeBundle materialises a one-concept bundle whose concept carries the given
// frontmatter lines, plus the reserved files a real bundle has.
func writeBundle(t *testing.T, fmLines string) string {
	t.Helper()
	dir := t.TempDir()
	must := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	must("index.md", "---\nokf_version: \"0.2\"\n---\n# idx\n\n## Assets\n\n- [c](c.md) - x\n")
	must("log.md", "# Change Log\n")
	must("c.md", "---\ntype: SQLite Table\ntitle: orders\ndescription: One row per completed customer order.\n"+
		fmLines+"---\n# Columns\n\n| id | INTEGER |\n")
	return dir
}

func gen(by string) string {
	return "generated:\n    by: " + by + "\n    at: \"2026-08-29T12:00:00Z\"\n"
}

func ver(by ...string) string {
	var b strings.Builder
	b.WriteString("verified:\n")
	for _, a := range by {
		b.WriteString("    - by: " + a + "\n      at: \"2026-08-29T12:05:00Z\"\n")
	}
	return b.String()
}

func scan(t *testing.T, fmLines string) []PolicyFinding {
	t.Helper()
	f, err := ScanPolicy(writeBundle(t, fmLines))
	if err != nil {
		t.Fatalf("ScanPolicy: %v", err)
	}
	return f
}

func TestPolicySelfSignFails(t *testing.T) {
	got := scan(t, gen(actorA)+ver(actorA))
	if len(got) != 1 {
		t.Fatalf("self-sign not caught: got %d findings, want 1 (%+v)", len(got), got)
	}
	if got[0].Rule != RulePolicySelfSign {
		t.Errorf("wrong rule id: %q", got[0].Rule)
	}
	if got[0].Path != "c.md" {
		t.Errorf("wrong path: %q", got[0].Path)
	}
}

func TestPolicyIndependentVerificationPasses(t *testing.T) {
	if got := scan(t, gen(actorA)+ver(actorB)); len(got) != 0 {
		t.Errorf("independent machine verification flagged: %+v", got)
	}
}

func TestPolicyHumanVerificationPasses(t *testing.T) {
	if got := scan(t, gen(actorA)+ver(human)); len(got) != 0 {
		t.Errorf("independent human verification flagged: %+v", got)
	}
}

func TestPolicyGeneratedOnlyPasses(t *testing.T) {
	if got := scan(t, gen(actorA)); len(got) != 0 {
		t.Errorf("concept with no verification flagged: %+v", got)
	}
}

// A self-signed entry is a violation even when an independent verifier is also
// present — the self-issued event still sits in the trust record.
func TestPolicySelfSignAlongsideIndependentFails(t *testing.T) {
	if got := scan(t, gen(actorA)+ver(actorA, human)); len(got) != 1 {
		t.Errorf("self-sign masked by an independent verifier: got %d, want 1 (%+v)", len(got), got)
	}
}

// Without `generated.by` there is no producer to compare against, so the gate
// stays silent rather than guessing.
func TestPolicyNoGeneratedPasses(t *testing.T) {
	if got := scan(t, ver(actorA)); len(got) != 0 {
		t.Errorf("concept without generated.by flagged: %+v", got)
	}
}

// Reserved files carry no verification state and must never be scanned.
func TestPolicySkipsReservedFiles(t *testing.T) {
	dir := writeBundle(t, gen(actorA)+ver(actorB))
	// index.md with a self-signed-looking frontmatter must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "index.md"),
		[]byte("---\nokf_version: \"0.2\"\n"+gen(actorA)+ver(actorA)+"---\n# idx\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ScanPolicy(dir)
	if err != nil {
		t.Fatalf("ScanPolicy: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("reserved file scanned: %+v", got)
	}
}

// A malformed concept is a conformance concern; the policy gate must not turn it
// into a policy violation or an error.
func TestPolicyIgnoresMalformedConcept(t *testing.T) {
	dir := writeBundle(t, gen(actorA)+ver(actorB))
	if err := os.WriteFile(filepath.Join(dir, "broken.md"), []byte("# no frontmatter\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ScanPolicy(dir)
	if err != nil {
		t.Fatalf("ScanPolicy returned an error for a malformed concept: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("malformed concept produced policy findings: %+v", got)
	}
}
