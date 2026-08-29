package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ConceptStructuralHash computes a stable hash over the *structural* content of a
// concept — the markdown body the connector regenerates deterministically each
// run — while deliberately excluding everything an agent or human owns:
// Frontmatter.Description, Frontmatter.Timestamp (which changes every run by
// construction), and Frontmatter.ContentHash itself (all live in frontmatter, not
// the body, so excluding the body's surroundings is automatic).
//
// The hash is computed from the FRESH (connector-generated) doc and stored in
// frontmatter; on the next run it is recomputed from the new fresh doc and
// compared to the stored value. Because a fresh doc never contains agent-authored
// prose, agent edits on disk never perturb the comparison — which is exactly what
// makes enrichment durable across re-produces.
//
// Line endings are normalized to "\n" and trailing whitespace is trimmed so the
// hash is stable regardless of platform checkout settings.
func ConceptStructuralHash(doc ConceptDoc) string {
	// Identity fields are connector-owned and deterministic, so a change in the
	// underlying asset's identity must invalidate the hash even when the
	// rendered body is unchanged.
	var b strings.Builder
	b.WriteString(doc.Frontmatter.Type)
	b.WriteByte(0)
	b.WriteString(doc.Frontmatter.Title)
	b.WriteByte(0)
	b.WriteString(doc.Frontmatter.Resource)
	b.WriteByte(0)
	b.WriteString(normalizeForHash(doc.Body))
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// normalizeForHash canonicalizes a body for hashing: CRLF/CR → LF, trailing
// whitespace stripped per line, and a single trailing newline.
func normalizeForHash(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	lines := strings.Split(body, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

// MergeConcept reconciles a freshly extracted concept with whatever already
// exists on disk, returning the document to write and whether it changed.
//
//   - existing == nil (new concept): fresh is returned with its ContentHash
//     stamped, changed == true.
//   - structure unchanged (existing.ContentHash == hash(fresh)): the existing doc
//     is returned verbatim with changed == false, so the caller skips the write
//     entirely — preserving the enriched description, any agent body prose, the
//     old timestamp, and the on-disk bytes/mtime.
//   - structure changed: a merged doc is returned (fresh structural body + new
//     ContentHash + new timestamp) that carries over agent-owned content: the
//     existing Description, the union of Tags, and any body sections the connector
//     did not regenerate (agent-added headings). changed == true.
func MergeConcept(existing *ConceptDoc, fresh ConceptDoc) (ConceptDoc, bool) {
	h := ConceptStructuralHash(fresh)
	fresh.Frontmatter.ContentHash = h

	if existing == nil {
		return fresh, true
	}
	if existing.Frontmatter.ContentHash == h {
		return *existing, false
	}

	merged := fresh
	if strings.TrimSpace(existing.Frontmatter.Description) != "" {
		merged.Frontmatter.Description = existing.Frontmatter.Description
	}
	// Carry the enrichment marker over. The new ContentHash will differ from it,
	// so okf-enrich correctly sees this concept as stale and re-enriches it.
	merged.Frontmatter.EnrichedAgainst = existing.Frontmatter.EnrichedAgainst
	merged.Frontmatter.Tags = unionTags(fresh.Frontmatter.Tags, existing.Frontmatter.Tags)
	// Carry agent/human-owned trust, provenance and lifecycle state that a
	// connector re-run does not regenerate. Fresh values win when present.
	merged.Frontmatter.Verified = mergeVerified(existing.Frontmatter.Verified, merged.Frontmatter.Verified)
	if len(merged.Frontmatter.Sources) == 0 {
		merged.Frontmatter.Sources = existing.Frontmatter.Sources
	}
	if merged.Frontmatter.UsageWindow == nil {
		merged.Frontmatter.UsageWindow = existing.Frontmatter.UsageWindow
	}
	if merged.Frontmatter.Status == "" {
		merged.Frontmatter.Status = existing.Frontmatter.Status
	}
	if merged.Frontmatter.StaleAfter == "" {
		merged.Frontmatter.StaleAfter = existing.Frontmatter.StaleAfter
	}
	// Extension keys merge key by key — the fresh (connector-owned) value wins a
	// collision, and a key only the existing doc carries is preserved. Replacing
	// the map wholesale would drop every agent- or policy-owned key the moment a
	// connector emits an extension key of its own. A fresh map is allocated so
	// neither input's map is aliased or mutated.
	if len(existing.Frontmatter.Extra) > 0 || len(merged.Frontmatter.Extra) > 0 {
		combined := make(map[string]interface{}, len(existing.Frontmatter.Extra)+len(merged.Frontmatter.Extra))
		for k, v := range existing.Frontmatter.Extra {
			combined[k] = v
		}
		for k, v := range merged.Frontmatter.Extra {
			combined[k] = v
		}
		merged.Frontmatter.Extra = combined
	}
	merged.Body = preserveExtraSections(fresh.Body, existing.Body)
	return merged, true
}

// mergeVerified concatenates two verification histories — existing entries first,
// then fresh entries not already present — and drops any entry whose (by, at)
// pair has already been seen.
//
// SPEC §5.2 treats multiple entries as independent checks, and (by, at) is the
// whole of a VerifiedEntry, so an exact pair match is the only event identity the
// format offers today. Replacing the list wholesale destroys an existing human
// sign-off the moment a producer emits a verification of its own, silently
// downgrading the derived trust tier. Order is stable concatenation rather than a
// sort, since `at` is optional and sorting an absent timestamp would be arbitrary.
func mergeVerified(existing, fresh VerifiedList) VerifiedList {
	if len(existing) == 0 && len(fresh) == 0 {
		return nil
	}
	type event struct{ by, at string }
	seen := make(map[event]bool, len(existing)+len(fresh))
	out := make(VerifiedList, 0, len(existing)+len(fresh))
	for _, list := range []VerifiedList{existing, fresh} {
		for _, e := range list {
			k := event{e.By, e.At}
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// unionTags returns the sorted, de-duplicated union of two tag slices, or nil if
// the result is empty (so omitempty drops the field).
func unionTags(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range append(append([]string{}, a...), b...) {
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// preserveExtraSections appends to freshBody any top-level (# / ##) sections that
// exist in existingBody but whose heading text is absent from freshBody. This
// keeps agent-added sections (notes, relationship prose under a new heading)
// across a structural re-produce, while connector-owned sections are taken from
// fresh.
func preserveExtraSections(freshBody, existingBody string) string {
	freshHeads := topLevelHeadings(freshBody)
	extras := extraSections(existingBody, freshHeads)
	if len(extras) == 0 {
		return freshBody
	}
	out := strings.TrimRight(freshBody, "\n")
	for _, sec := range extras {
		out += "\n\n" + strings.TrimRight(sec, "\n")
	}
	return out + "\n"
}

// topLevelHeadings returns the set of heading texts for level-1/level-2 ATX
// headings in body (e.g. "Columns" for "# Columns").
func topLevelHeadings(body string) map[string]bool {
	heads := map[string]bool{}
	for _, ln := range strings.Split(body, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "## ") {
			heads[strings.TrimSpace(t[3:])] = true
		} else if strings.HasPrefix(t, "# ") {
			heads[strings.TrimSpace(t[2:])] = true
		}
	}
	return heads
}

// extraSections returns the full text of each level-1/2 section in body whose
// heading text is NOT in known, preserving each section from its heading up to
// the next level-1/2 heading.
func extraSections(body string, known map[string]bool) []string {
	lines := strings.Split(body, "\n")
	var sections []string
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		var head string
		if strings.HasPrefix(t, "## ") {
			head = strings.TrimSpace(t[3:])
		} else if strings.HasPrefix(t, "# ") {
			head = strings.TrimSpace(t[2:])
		} else {
			continue
		}
		// Find the section end.
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			jt := strings.TrimSpace(lines[j])
			if strings.HasPrefix(jt, "# ") || strings.HasPrefix(jt, "## ") {
				end = j
				break
			}
		}
		if !known[head] {
			sections = append(sections, strings.Join(lines[i:end], "\n"))
		}
		i = end - 1
	}
	return sections
}
