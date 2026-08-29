package okf

// G-Core boundary-requirement regression suite.
//
// These tests encode the 13 boundary requirements this fork exists to satisfy.
// They are deliberately written against the exported API only, so they keep
// working across upstream refactors, and they are the gate for any rebase onto
// upstream: if a merge from xSAVIKx/okf-skills makes one of these fail, the
// fork's reason for existing has regressed.
//
// Baseline on upstream @9740e893 (unpatched): 10 pass / 8 fail.
// See ../FORK.md for which patch restores which case.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func boundaryWrite(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

const boundaryConformantDoc = `---
type: SQLite Table
title: 주문
description: 채널 전체 주문.
resource: sqlite:///shop.db#orders
tags:
    - core
generated:
    by: okf-sqlite/0.9.0
    at: "2026-08-01T10:00:00Z"
verified:
    - by: human:vavagirls
      at: "2026-08-02T11:00:00Z"
sources:
    - id: shop-db
      resource: sqlite:///shop.db
      title: Shop DB
      author: team:data
      usage_count: 5000
status: stable
stale_after: "2026-12-31"
content_hash: abc123
gcore_evidence_id: EV-77281
custom_block:
    reviewer: kim
---

# 주문 🎯 "q"
`

// boundaryRoundTrip reads the conformant fixture and writes it back out.
func boundaryRoundTrip(t *testing.T) string {
	t.Helper()
	src := boundaryWrite(t, "in.md", boundaryConformantDoc)
	doc, err := ReadConceptDoc(src)
	if err != nil {
		t.Fatalf("ReadConceptDoc: %v", err)
	}
	out := filepath.Join(filepath.Dir(src), "out.md")
	if err := WriteConceptDoc(out, *doc); err != nil {
		t.Fatalf("WriteConceptDoc: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	return string(b)
}

// B1: encoding fidelity — non-ASCII, emoji and quotes survive a round-trip.
func TestBoundaryEncodingFidelity(t *testing.T) {
	got := boundaryRoundTrip(t)
	for _, want := range []string{`주문 🎯 "q"`, "채널 전체 주문."} {
		if !strings.Contains(got, want) {
			t.Errorf("round-trip lost %q", want)
		}
	}
}

// B2: every spec-defined field survives a round-trip.
func TestBoundarySpecFieldRoundTrip(t *testing.T) {
	got := boundaryRoundTrip(t)
	for _, want := range []string{
		"type:", "title:", "resource:", "generated:", "verified:", "sources:",
		"status: stable", "stale_after:", "content_hash:",
		"usage_count: 5000", "author: team:data",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("round-trip dropped spec field %q", want)
		}
	}
}

// B3: SPEC v0.2 §4.1 — consumers SHOULD preserve unknown keys when round-tripping.
func TestBoundaryUnknownKeyPreservation(t *testing.T) {
	got := boundaryRoundTrip(t)
	for _, want := range []string{"gcore_evidence_id", "custom_block", "reviewer"} {
		if !strings.Contains(got, want) {
			t.Errorf("round-trip dropped producer-defined key %q", want)
		}
	}
}

const (
	boundaryBodyV1 = "## Columns\n\n| c | t |\n| --- | --- |\n| id | INT |\n"
	boundaryBodyV2 = boundaryBodyV1 + "| total | REAL |\n"
)

// boundaryStructuralChange builds an enriched, human-verified concept and merges
// a connector re-run whose body changed structurally.
func boundaryStructuralChange() (ConceptDoc, bool) {
	existing := ConceptDoc{Frontmatter: Frontmatter{
		Type: "SQLite Table", Title: "orders", Resource: "sqlite:///A#orders",
		Description: "사람이 다듬은 설명.",
		Verified:    VerifiedList{{By: "human:vavagirls", At: "2026-08-02T11:00:00Z"}},
		Sources:     []SourceEntry{{ID: "db", Resource: "sqlite:///A", Author: "team:data", UsageCount: 4200}},
		UsageWindow: &UsageWindow{From: "2026-06-01T00:00:00Z", To: "2026-06-30T00:00:00Z"},
		Status:      "stable", StaleAfter: "2026-12-31",
	}, Body: boundaryBodyV1}
	existing.Frontmatter.ContentHash = ConceptStructuralHash(existing)

	fresh := ConceptDoc{Frontmatter: Frontmatter{
		Type: "SQLite Table", Title: "orders", Resource: "sqlite:///A#orders",
		Generated: &GeneratedInfo{By: "okf-sqlite/0.9.0", At: "2026-08-29T09:00:00Z"},
	}, Body: boundaryBodyV2}

	return func() ConceptDoc { m, _ := MergeConcept(&existing, fresh); return m }(),
		func() bool { _, c := MergeConcept(&existing, fresh); return c }()
}

// B4: an agent- or human-authored description survives a structural re-produce.
func TestBoundaryEnrichmentPreserved(t *testing.T) {
	m, _ := boundaryStructuralChange()
	if m.Frontmatter.Description != "사람이 다듬은 설명." {
		t.Errorf("description not preserved: %q", m.Frontmatter.Description)
	}
}

// B5: provenance credibility signals survive a structural re-produce.
func TestBoundaryProvenancePreserved(t *testing.T) {
	m, _ := boundaryStructuralChange()
	if len(m.Frontmatter.Sources) != 1 {
		t.Fatalf("sources dropped: got %d, want 1", len(m.Frontmatter.Sources))
	}
	s := m.Frontmatter.Sources[0]
	if s.Author != "team:data" || s.UsageCount != 4200 {
		t.Errorf("source credibility signals lost: %+v", s)
	}
}

// B6: human verification survives a structural re-produce.
// SPEC §5.2: verified is independent of generated.at — content can change
// without re-confirmation, so a re-produce must not silently clear it.
func TestBoundaryTrustStatePreserved(t *testing.T) {
	m, _ := boundaryStructuralChange()
	if len(m.Frontmatter.Verified) != 1 || m.Frontmatter.Verified[0].By != "human:vavagirls" {
		t.Fatalf("verification dropped: %+v", m.Frontmatter.Verified)
	}
	if got := m.Frontmatter.GetTrustTier(); got != TrustTierHumanReviewed {
		t.Errorf("trust tier silently downgraded to %q", got)
	}
}

// B6b: lifecycle state survives a structural re-produce.
func TestBoundaryLifecyclePreserved(t *testing.T) {
	m, _ := boundaryStructuralChange()
	if m.Frontmatter.Status != "stable" {
		t.Errorf("status dropped: %q", m.Frontmatter.Status)
	}
	if m.Frontmatter.StaleAfter != "2026-12-31" {
		t.Errorf("stale_after dropped: %q", m.Frontmatter.StaleAfter)
	}
	if m.Frontmatter.UsageWindow == nil {
		t.Error("usage_window dropped")
	}
}

// B7: an unchanged structure must report changed==false so the caller skips
// the write entirely (no byte or mtime churn).
func TestBoundaryIncrementalNoChurn(t *testing.T) {
	fresh := ConceptDoc{Frontmatter: Frontmatter{
		Type: "SQLite Table", Title: "orders", Resource: "sqlite:///A#orders",
	}, Body: boundaryBodyV1}
	existing := fresh
	existing.Frontmatter.ContentHash = ConceptStructuralHash(fresh)

	if _, changed := MergeConcept(&existing, fresh); changed {
		t.Error("unchanged structure reported as changed")
	}
}

// B8: relationship rendering is order-independent.
func TestBoundaryRelationshipDeterminism(t *testing.T) {
	a := []Relationship{
		{Label: "FK on customer_id", Target: "/t/customers.md", Text: "customers"},
		{Label: "FK on item_id", Target: "/t/items.md", Text: "items"},
		{Label: "FK on a", Target: "/t/aaa.md", Text: "aaa"},
	}
	b := []Relationship{a[2], a[0], a[1]}
	if RenderRelationshipsSection(a) != RenderRelationshipsSection(b) {
		t.Error("relationship rendering depends on input order")
	}
}

// B9: the structural hash is stable across calls.
func TestBoundaryHashDeterminism(t *testing.T) {
	d := ConceptDoc{Frontmatter: Frontmatter{Type: "T", Title: "x"}, Body: boundaryBodyV1}
	if ConceptStructuralHash(d) != ConceptStructuralHash(d) {
		t.Error("structural hash is not deterministic")
	}
}

// B9b: a change of the underlying asset's identity must invalidate the hash even
// when the rendered body is byte-identical. Otherwise MergeConcept reports
// changed==false, the caller skips the write, and the concept keeps describing
// the wrong resource with no trace in the bundle.
func TestBoundaryIdentityChangeDetected(t *testing.T) {
	a := ConceptDoc{Frontmatter: Frontmatter{
		Type: "SQLite Table", Title: "orders", Resource: "sqlite:///PROD-A.db#orders",
	}, Body: boundaryBodyV1}
	b := ConceptDoc{Frontmatter: Frontmatter{
		Type: "PostgreSQL Table", Title: "orders_v2", Resource: "postgres://PROD-B/orders_v2",
	}, Body: boundaryBodyV1}

	if ConceptStructuralHash(a) == ConceptStructuralHash(b) {
		t.Error("identity change (type/title/resource) did not invalidate the structural hash")
	}
}

// B10/B10b/B10c/B13: staleness must honour the canonical v0.2 form
// (`stale_after` is an absolute instant, SPEC §5.5), keep accepting the
// date-only form, and never absorb a parse failure into "fresh".
func TestBoundaryStaleness(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		value string
		stale bool
	}{
		{"canonical instant, past", "2020-01-01T00:00:00Z", true},
		{"canonical instant, future", "2030-01-01T00:00:00Z", false},
		{"date only, past", "2020-01-01", true},
		{"date only, future", "2030-01-01", false},
		{"absent", "", false},
		{"unparseable fails closed", "not-a-date", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := (Frontmatter{StaleAfter: c.value}).IsStale(now); got != c.stale {
				t.Errorf("IsStale(%q) = %v, want %v", c.value, got, c.stale)
			}
		})
	}
}

// B11/B11b: malformed input fails closed rather than yielding a partial document.
func TestBoundaryMalformedInputFailsClosed(t *testing.T) {
	if _, err := ReadConceptDoc(boundaryWrite(t, "nofm.md", "# no frontmatter\n")); err == nil {
		t.Error("missing frontmatter accepted without error")
	}
	bad := "---\ntype: T\ntitle: \"unclosed\ntags: [a\n---\n# b\n"
	if _, err := ReadConceptDoc(boundaryWrite(t, "bad.md", bad)); err == nil {
		t.Error("malformed YAML accepted without error")
	}
}

// B12: a `verified` value of an unexpected shape must be surfaced, not silently
// discarded — a silent drop destroys the verification record on the next write.
func TestBoundaryMalformedTrustSurfaced(t *testing.T) {
	p := boundaryWrite(t, "scalar.md", "---\ntype: T\nverified: human:vavagirls\n---\n# b\n")
	doc, err := ReadConceptDoc(p)
	if err == nil && doc != nil && len(doc.Frontmatter.Verified) == 0 {
		t.Error("malformed `verified` was silently dropped with no error")
	}
}
