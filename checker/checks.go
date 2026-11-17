package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const toolName = "trust_check"

type Check struct {
	ID     string `json:"id"`
	Status string `json:"status"` // PASS | FAIL | SKIP
	Detail string `json:"detail"`
}

type ClaimRef struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	Verification string `json:"verification,omitempty"`
	Control      string `json:"control,omitempty"`
}

type Report struct {
	Tool          string     `json:"tool"`
	Version       string     `json:"version"`
	CheckedAt     string     `json:"checked_at"`
	Root          string     `json:"root"`
	CheckedCommit string     `json:"checked_commit,omitempty"` // HEAD when the root is a git work tree
	Overall       string     `json:"overall"`                  // PASS | FAIL
	Claims        []ClaimRef `json:"claims"`
	Checks        []Check    `json:"checks"`
}

type disclosureItem struct {
	ID     string  `json:"id"`
	Path   string  `json:"path"`
	SHA256 *string `json:"sha256"`
}

type metricsContract struct {
	OnStatus     string   `json:"on_status"`
	RequiredKeys []string `json:"required_keys"`
}

type manifestFile struct {
	CriteriaLock struct {
		Commit string `json:"commit"`
		Date   string `json:"date"`
		Note   string `json:"note"`
	} `json:"criteria_lock"`
	Object struct {
		Commit string `json:"commit"`
	} `json:"object"`
	Evidence []struct {
		ID     string  `json:"id"`
		Type   string  `json:"type"`
		Path   string  `json:"path"`
		SHA256 *string `json:"sha256"`
	} `json:"evidence"`
	Disclosures []disclosureItem `json:"disclosures"`
	// Optional, manifest-declared contracts (data, not code: tc stays
	// domain-agnostic). audit_metrics_contract names, per audit, which
	// metric keys a run-output report must carry when the audit ends in
	// on_status. external_references whitelists verdict-like ids cited by
	// the record but living outside it.
	AuditMetricsContract map[string]metricsContract `json:"audit_metrics_contract"`
	ExternalReferences   []string                   `json:"external_references"`
	Controls             []struct {
		ID string `json:"id"`
		N  int    `json:"N"`
	} `json:"controls"`
}

type claimFile struct {
	ID           string   `json:"id"`
	Status       string   `json:"status"`
	Verification string   `json:"verification"`
	Evidence     []string `json:"evidence"`
	Control      string   `json:"control"`
	ReasonCode   string   `json:"reason_code"`
	// Optional structured decision metadata (v0.2.0): what the audit
	// battery decided for the submitted case. Fields are optional but
	// must appear together, and must match the referenced run-output
	// audit report (AUDIT_DECISION_XREF).
	CaseVerdict  *string   `json:"case_verdict"`
	CaseReason   *string   `json:"case_reason_code"`
	Failing      *[]string `json:"failing_audits"`
	Inconclusive *[]string `json:"inconclusive_audits"`
}

type verdictFile struct {
	VerdictID     string      `json:"verdict_id"`
	Date          string      `json:"date"`
	ValidUntil    string      `json:"valid_until"`
	ObjectVersion string      `json:"object_version"`
	Claims        []claimFile `json:"claims"`
}

var claimHeading = regexp.MustCompile(`(?m)^##\s+(C-\d+)`)

func writeReport(path string, rep *Report) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

type reportBuilder struct{ checks []Check }

func (r *reportBuilder) add(id, status, detail string) {
	r.checks = append(r.checks, Check{ID: id, Status: status, Detail: detail})
}

func (r *reportBuilder) failIf(cond bool, id, okDetail, failDetail string) {
	if cond {
		r.add(id, "FAIL", failDetail)
	} else {
		r.add(id, "PASS", okDetail)
	}
}

// runChecks validates the mechanical integrity of a Trust Record located at
// <root>/trust/. It never judges meaning: a green report states "the
// mechanics hold", nothing more.
func runChecks(root string) (*Report, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	rep := &Report{
		Tool:      toolName,
		Version:   version,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Root:      abs,
	}
	r := &reportBuilder{}
	trustDir := filepath.Join(abs, "trust")

	// 1. presence & non-emptiness of the four record files
	files := []string{"claims.md", "evidence-manifest.json", "verdict.json", "limitations.md"}
	var missing []string
	for _, f := range files {
		fi, err := os.Stat(filepath.Join(trustDir, f))
		if err != nil || fi.Size() == 0 {
			missing = append(missing, f)
		}
	}
	r.failIf(len(missing) > 0, "FILES_PRESENT",
		fmt.Sprintf("all %d record files present and non-empty", len(files)),
		"missing/empty: "+strings.Join(missing, ", "))

	// 2. JSON validity and minimal structure
	manBytes, errMan := os.ReadFile(filepath.Join(trustDir, "evidence-manifest.json"))
	verBytes, errVer := os.ReadFile(filepath.Join(trustDir, "verdict.json"))
	var man manifestFile
	var ver verdictFile
	jsonOk := true
	jsonState := "manifest and verdict parse; structure complete"
	switch {
	case errMan != nil:
		jsonOk, jsonState = false, "evidence-manifest.json unreadable: "+errMan.Error()
	case errVer != nil:
		jsonOk, jsonState = false, "verdict.json unreadable: "+errVer.Error()
	default:
		if e := json.Unmarshal(manBytes, &man); e != nil {
			jsonOk, jsonState = false, "evidence-manifest.json invalid: "+e.Error()
		} else if e := json.Unmarshal(verBytes, &ver); e != nil {
			jsonOk, jsonState = false, "verdict.json invalid: "+e.Error()
		} else if len(man.Evidence) == 0 {
			jsonOk, jsonState = false, "manifest lists no evidence"
		} else if len(ver.Claims) == 0 {
			jsonOk, jsonState = false, "verdict lists no claims"
		} else if strings.TrimSpace(man.CriteriaLock.Commit) == "" {
			jsonOk, jsonState = false, "criteria_lock.commit missing"
		}
	}
	r.failIf(!jsonOk, "JSON_VALID", jsonState, jsonState)

	if jsonOk {
		// 3. sha256 of hashed evidence artifacts
		var bad, skipped []string
		verified, verifiedDisclosures := 0, 0
		for _, ev := range man.Evidence {
			if ev.Path == "" || ev.SHA256 == nil || strings.TrimSpace(*ev.SHA256) == "" {
				skipped = append(skipped, ev.ID)
				continue
			}
			data, err := os.ReadFile(filepath.Join(abs, filepath.FromSlash(ev.Path)))
			if err != nil {
				bad = append(bad, fmt.Sprintf("%s: unreadable: %v", ev.ID, err))
				continue
			}
			sum := sha256.Sum256(data)
			got := hex.EncodeToString(sum[:])
			if !strings.EqualFold(got, strings.TrimSpace(*ev.SHA256)) {
				bad = append(bad, fmt.Sprintf("%s: declared %s..., actual %s...", ev.ID, shortHash(*ev.SHA256), shortHash(got)))
				continue
			}
			verified++
		}
		for _, d := range man.Disclosures {
			if d.Path == "" || d.SHA256 == nil || strings.TrimSpace(*d.SHA256) == "" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(abs, filepath.FromSlash(d.Path)))
			if err != nil {
				bad = append(bad, fmt.Sprintf("%s: unreadable: %v", d.ID, err))
				continue
			}
			sum := sha256.Sum256(data)
			if !strings.EqualFold(hex.EncodeToString(sum[:]), strings.TrimSpace(*d.SHA256)) {
				bad = append(bad, fmt.Sprintf("%s: declared %s..., actual %s...",
					d.ID, shortHash(*d.SHA256), shortHash(hex.EncodeToString(sum[:]))))
				continue
			}
			verifiedDisclosures++
		}
		status, detail := "PASS", fmt.Sprintf("%d hashed artifacts verified", verified)
		if verifiedDisclosures > 0 {
			detail = fmt.Sprintf("%d hashed artifacts verified (%d evidence + %d disclosures)",
				verified+verifiedDisclosures, verified, verifiedDisclosures)
		}
		if len(skipped) > 0 {
			detail += fmt.Sprintf("; %d skipped (no hash by design: %s)", len(skipped), strings.Join(skipped, ","))
		}
		if len(bad) > 0 {
			status, detail = "FAIL", detail+"; MISMATCH: "+strings.Join(bad, "; ")
		}
		r.add("HASHES_MATCH", status, detail)

		// 4. every verdict claim id exists in claims.md
		claimsMD, err := os.ReadFile(filepath.Join(trustDir, "claims.md"))
		if err != nil {
			r.add("CLAIMS_XREF", "FAIL", "claims.md unreadable: "+err.Error())
		} else {
			known := map[string]bool{}
			for _, m := range claimHeading.FindAllStringSubmatch(string(claimsMD), -1) {
				known[m[1]] = true
			}
			var absent []string
			for _, c := range ver.Claims {
				if !known[c.ID] {
					absent = append(absent, c.ID)
				}
			}
			r.failIf(len(absent) > 0, "CLAIMS_XREF",
				fmt.Sprintf("all %d claim ids present in claims.md", len(ver.Claims)),
				"in verdict but absent in claims.md: "+strings.Join(absent, ", "))
		}

		// 5. every evidence id referenced by verdict exists in manifest
		evIDs := map[string]bool{}
		for _, ev := range man.Evidence {
			evIDs[ev.ID] = true
		}
		var dangling []string
		for _, c := range ver.Claims {
			for _, e := range c.Evidence {
				if !evIDs[e] {
					dangling = append(dangling, c.ID+"->"+e)
				}
			}
		}
		r.failIf(len(dangling) > 0, "EVIDENCE_XREF",
			"all evidence references resolve to manifest entries",
			"dangling evidence references: "+strings.Join(dangling, ", "))

		// 6. statistical PASS claims carry a negative control with N >= 1
		ctrl := map[string]int{}
		for _, c := range man.Controls {
			ctrl[c.ID] = c.N
		}
		var ctrlIssues []string
		for _, c := range ver.Claims {
			if c.Status != "PASS" {
				continue
			}
			if c.Control != "" {
				n, ok := ctrl[c.Control]
				if !ok {
					ctrlIssues = append(ctrlIssues, c.ID+": control "+c.Control+" not defined in manifest")
				} else if n < 1 {
					ctrlIssues = append(ctrlIssues, fmt.Sprintf("%s: control %s has N < 1", c.ID, c.Control))
				}
			} else if strings.Contains(c.Verification, "V-02") {
				ctrlIssues = append(ctrlIssues, c.ID+": statistical PASS (verification references V-02) without negative control")
			}
		}
		r.failIf(len(ctrlIssues) > 0, "CONTROLS",
			fmt.Sprintf("controls sound for %d PASS claims", countPASS(ver.Claims)),
			strings.Join(ctrlIssues, "; "))

		// 7. FAIL / INCONCLUSIVE claims carry a reason_code
		var noReason []string
		for _, c := range ver.Claims {
			if (c.Status == "FAIL" || c.Status == "INCONCLUSIVE") && strings.TrimSpace(c.ReasonCode) == "" {
				noReason = append(noReason, c.ID)
			}
		}
		r.failIf(len(noReason) > 0, "REASON_CODES",
			"every non-PASS claim carries a reason_code",
			"FAIL/INCONCLUSIVE without reason_code: "+strings.Join(noReason, ", "))

		// 8. verdict validity horizon
		r.failIf(strings.TrimSpace(ver.ValidUntil) == "", "VALID_UNTIL",
			"valid_until: "+ver.ValidUntil,
			"verdict.valid_until is empty")

		// 8a. run-output audit reports: collect + AUDIT_SHAPE (v0.2.0).
		// An "audit report" is any evidence item that parses as JSON and embeds
		// a case verdict with the audit battery's per-audit outcomes (a per-case
		// JSON of a track-record run).
		reports := collectAuditReports(abs, &man)
		reportByEvidence := map[string]*auditReport{}
		for i := range reports {
			reportByEvidence[reports[i].evidenceID] = &reports[i]
		}

		// AUDIT_SHAPE: serialization contract of the audit battery's own output.
		// Statuses from frozen enums; ok-entries carry reason_code "" plus a
		// string detail and an object metrics; non-ok entries cite the frozen
		// reason-code vocabulary. Catches positional/serialization corruption
		// of the audit output (the class that corrupted per-case evidence).
		if len(reports) == 0 {
			r.add("AUDIT_SHAPE", "SKIP", "no run-output audit reports among the evidence")
		} else {
			var problems []string
			caseStatuses := map[string]bool{"PASS": true, "FAIL": true, "INCONCLUSIVE": true}
			for _, rep := range reports {
				if !caseStatuses[rep.status] {
					problems = append(problems, fmt.Sprintf("%s: verdict.status %q not in {PASS, FAIL, INCONCLUSIVE}", rep.evidenceID, rep.status))
				}
				for _, ae := range rep.audits {
					name := str(ae.Name)
					loc := fmt.Sprintf("%s[%s]", rep.evidenceID, name)
					if name == "" {
						problems = append(problems, loc+": audit entry without a name")
						continue
					}
					st := str(ae.Status)
					if !auditStatuses[st] {
						problems = append(problems, fmt.Sprintf("%s: status %q not in {ok, fail, inconclusive, skipped}", loc, st))
						continue
					}
					if ae.ReasonCode == nil {
						problems = append(problems, loc+": missing reason_code field")
					} else if st == "ok" && *ae.ReasonCode != "" {
						problems = append(problems, fmt.Sprintf("%s: ok-audit carries reason_code %q (must be empty; narrative belongs in detail)", loc, *ae.ReasonCode))
					} else if st != "ok" && !reasonCodes[*ae.ReasonCode] {
						problems = append(problems, fmt.Sprintf("%s: reason_code %q outside the frozen CV0-TrustLayer-v3.5 vocabulary", loc, *ae.ReasonCode))
					}
					if ae.Detail == nil {
						problems = append(problems, loc+": missing detail field")
					}
					if ae.Metrics == nil {
						problems = append(problems, loc+": missing metrics object")
					}
				}
			}
			if len(problems) > 0 {
				r.add("AUDIT_SHAPE", "FAIL", fmt.Sprintf("%d audit report(s), %d shape violation(s): %s",
					len(reports), len(problems), strings.Join(problems, "; ")))
			} else {
				r.add("AUDIT_SHAPE", "PASS", fmt.Sprintf("%d run-output audit report(s) satisfy the serialization contract", len(reports)))
			}
		}

		// AUDIT_DECISION_XREF (v0.2.0): optional structured decision metadata on
		// verdict claims (what the battery decided for the submitted case) must
		// be complete (all four fields together — inconclusive audits may not be
		// silently dropped) and must match the referenced run-output report.
		// Control claims verify controls, not a case report, and are exempt.
		{
			var problems []string
			checked := 0
			for _, c := range ver.Claims {
				hasAny := c.CaseVerdict != nil || c.CaseReason != nil || c.Failing != nil || c.Inconclusive != nil
				if !hasAny {
					continue
				}
				if c.CaseVerdict == nil || c.CaseReason == nil || c.Failing == nil || c.Inconclusive == nil {
					problems = append(problems, c.ID+": partial decision metadata (case_verdict, case_reason_code, failing_audits and inconclusive_audits are required together)")
					continue
				}
				if strings.TrimSpace(c.Control) != "" {
					continue
				}
				var rep *auditReport
				matches := 0
				for _, id := range c.Evidence {
					if rpt, ok := reportByEvidence[id]; ok {
						matches++
						rep = rpt
					}
				}
				if matches == 0 {
					problems = append(problems, c.ID+": decision metadata references no run-output audit report in its evidence")
					continue
				}
				if matches > 1 {
					problems = append(problems, c.ID+": ambiguous decision metadata: multiple audit reports referenced")
					continue
				}
				checked++
				if *c.CaseVerdict != rep.status {
					problems = append(problems, fmt.Sprintf("%s: case_verdict %q but report %s says %q", c.ID, *c.CaseVerdict, rep.evidenceID, rep.status))
				}
				if *c.CaseReason != rep.reasonCode {
					problems = append(problems, fmt.Sprintf("%s: case_reason_code %q but report %s says %q", c.ID, *c.CaseReason, rep.evidenceID, rep.reasonCode))
				}
				var fails, inconv []string
				for _, ae := range rep.audits {
					st := str(ae.Status)
					switch st {
					case "fail":
						fails = append(fails, str(ae.Name))
					case "inconclusive":
						inconv = append(inconv, str(ae.Name))
					}
				}
				if !equalStrings(sortedCopy(*c.Failing), sortedCopy(fails)) {
					problems = append(problems, fmt.Sprintf("%s: failing_audits %v but report %s has %v", c.ID, *c.Failing, rep.evidenceID, fails))
				}
				if !equalStrings(sortedCopy(*c.Inconclusive), sortedCopy(inconv)) {
					problems = append(problems, fmt.Sprintf("%s: inconclusive_audits %v but report %s has %v", c.ID, *c.Inconclusive, rep.evidenceID, inconv))
				}
			}
			switch {
			case len(problems) > 0:
				r.add("AUDIT_DECISION_XREF", "FAIL", strings.Join(problems, "; "))
			case checked == 0:
				r.add("AUDIT_DECISION_XREF", "SKIP", "no claims carry structured decision metadata")
			default:
				r.add("AUDIT_DECISION_XREF", "PASS", fmt.Sprintf("decision metadata of %d claim(s) matches the run-output audit reports", checked))
			}
		}

		// CLAIM_PROMISE (v0.2.0): pre-run audit promises live in the LOCKED
		// claims.md as marker lines —
		//     Promise: case=<case_id> audits=<a1,a2,...> evidence=<E-ID>
		// — never in post-hoc verdict fields: a promise the battery can fill in
		// after the run is not a promise. Each promised audit must end with
		// status "fail" in the referenced run-output report.
		{
			promises, perr := parsePromises(string(claimsMD))
			switch {
			case len(promises) == 0 && len(perr) == 0:
				r.add("CLAIM_PROMISE", "SKIP", "claims.md declares no 'Promise:' lines (optional pre-run audit promises)")
			case len(perr) > 0:
				r.add("CLAIM_PROMISE", "FAIL", "malformed Promise declarations: "+strings.Join(perr, "; "))
			default:
				var problems []string
				evPath := map[string]string{}
				for _, ev := range man.Evidence {
					evPath[ev.ID] = ev.Path
				}
				for _, p := range promises {
					path, ok := evPath[p.evidence]
					if !ok {
						problems = append(problems, fmt.Sprintf("claims.md:%d: Promise evidence %q is not a manifest evidence id", p.line, p.evidence))
						continue
					}
					rep, isReport := reportByEvidence[p.evidence]
					if !isReport {
						problems = append(problems, fmt.Sprintf("claims.md:%d: Promise evidence %q (%s) is not a run-output audit report", p.line, p.evidence, path))
						continue
					}
					if rep.caseID != "" && p.caseID != rep.caseID {
						problems = append(problems, fmt.Sprintf("claims.md:%d: Promise case %q but report %s has case_id %q", p.line, p.caseID, p.evidence, rep.caseID))
					}
					byName := map[string]string{}
					for _, ae := range rep.audits {
						byName[str(ae.Name)] = str(ae.Status)
					}
					for _, a := range p.audits {
						st, ok := byName[a]
						if !ok {
							problems = append(problems, fmt.Sprintf("claims.md:%d: promised audit %q absent from report %s", p.line, a, p.evidence))
							continue
						}
						if st != "fail" {
							problems = append(problems, fmt.Sprintf("claims.md:%d: promised audit %q has status %q in report %s, expected fail", p.line, a, st, p.evidence))
						}
					}
				}
				if len(problems) > 0 {
					r.add("CLAIM_PROMISE", "FAIL", "pre-registered audit promises violated: "+strings.Join(problems, "; "))
				} else {
					r.add("CLAIM_PROMISE", "PASS", fmt.Sprintf("%d pre-registered audit promise(s) hold in the run-output evidence", len(promises)))
				}
			}
		}

		// REQUIRED_METRICS (v0.2.0): manifest-declared contract (data, not code —
		// tc stays domain-agnostic): per audit name, which metric keys a report
		// must carry when the audit ends in a given status. Absent contract ->
		// SKIP.
		{
			if len(man.AuditMetricsContract) == 0 {
				r.add("REQUIRED_METRICS", "SKIP", "manifest declares no audit_metrics_contract")
			} else {
				var problems []string
				checked := 0
				for _, rep := range reports {
					for _, ae := range rep.audits {
						name := str(ae.Name)
						c, declared := man.AuditMetricsContract[name]
						if !declared || str(ae.Status) != c.OnStatus {
							continue
						}
						checked++
						have := map[string]bool{}
						for k := range ae.Metrics {
							have[k] = true
						}
						for _, key := range c.RequiredKeys {
							if !have[key] {
								problems = append(problems, fmt.Sprintf("%s[%s] (status %s): missing metric key %q per manifest contract",
									rep.evidenceID, name, c.OnStatus, key))
							}
						}
					}
				}
				if len(problems) > 0 {
					r.add("REQUIRED_METRICS", "FAIL", "manifest metric contract violated: "+strings.Join(problems, "; "))
				} else {
					r.add("REQUIRED_METRICS", "PASS", fmt.Sprintf("manifest metric contract holds (%d audit outcome(s) checked)", checked))
				}
			}
		}

		// EVIDENCE_SECTION (v0.2.0): content that is NOT evidence (pre-lock
		// calibration, drafts) must live in disclosures[], never in evidence[].
		if len(man.Disclosures) == 0 {
			r.add("EVIDENCE_SECTION", "SKIP", "manifest declares no disclosures section")
		} else {
			var problems []string
			evIDs := map[string]bool{}
			for _, ev := range man.Evidence {
				evIDs[ev.ID] = true
			}
			for _, d := range man.Disclosures {
				if strings.TrimSpace(d.ID) == "" {
					problems = append(problems, "disclosure item without an id")
					continue
				}
				if evIDs[d.ID] {
					problems = append(problems, fmt.Sprintf("disclosure %q collides with an evidence id: non-evidence content must live outside evidence[]", d.ID))
				}
				if strings.TrimSpace(d.Path) == "" {
					problems = append(problems, fmt.Sprintf("disclosure %q has no path", d.ID))
				}
				if d.SHA256 == nil || strings.TrimSpace(*d.SHA256) == "" {
					problems = append(problems, fmt.Sprintf("disclosure %q has no sha256 (disclosed files are hashed too)", d.ID))
				}
			}
			if len(problems) > 0 {
				r.add("EVIDENCE_SECTION", "FAIL", strings.Join(problems, "; "))
			} else {
				r.add("EVIDENCE_SECTION", "PASS", fmt.Sprintf("%d disclosure item(s) kept outside the evidence chain", len(man.Disclosures)))
			}
		}

		// Shared text corpus for the reference/hygiene checks (v0.2.0).
		texts := map[string]string{
			"claims.md":              string(claimsMD),
			"evidence-manifest.json": string(manBytes),
			"verdict.json":           string(verBytes),
		}
		if lim, err := os.ReadFile(filepath.Join(trustDir, "limitations.md")); err == nil {
			texts["limitations.md"] = string(lim)
		}

		// EXTERNAL_REF_WHITELIST (v0.2.0): verdict-like ids cited by the record
		// must resolve within it: the record's own verdict id, the supersedes
		// chain, explicit ranges (V-X-001..004), or the manifest's
		// external_references whitelist.
		{
			allowed := externalRefsAllowed(ver.VerdictID, &man, texts)
			var unexpected []string
			seen := map[string]bool{}
			for _, text := range texts {
				for _, id := range verdictIDRe.FindAllString(text, -1) {
					if !allowed[id] && !seen[id] {
						seen[id] = true
						unexpected = append(unexpected, id)
					}
				}
			}
			sort.Strings(unexpected)
			if len(unexpected) > 0 {
				r.add("EXTERNAL_REF_WHITELIST", "FAIL", fmt.Sprintf("record references not resolvable within the record: %s; declare them in manifest.external_references or drop them", strings.Join(unexpected, ", ")))
			} else {
				r.add("EXTERNAL_REF_WHITELIST", "PASS", "all record references resolve within the record (own id, supersedes chain, ranges, whitelist)")
			}
		}

		// PATH_HYGIENE (v0.2.0): machine-local paths and personal emails must
		// not leak into published record files.
		{
			var leaks []string
			homeRe := regexp.MustCompile(`(/home/[A-Za-z0-9_.\-]+|/Users/[A-Za-z0-9_.\-]+)`)
			emailRe := regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
			for name, text := range texts {
				for _, m := range homeRe.FindAllString(text, -1) {
					leaks = append(leaks, fmt.Sprintf("%s: machine-local path %s", name, m))
				}
				for _, m := range emailRe.FindAllString(text, -1) {
					if !strings.HasSuffix(m, "@users.noreply.github.com") {
						leaks = append(leaks, fmt.Sprintf("%s: personal email %s", name, m))
					}
				}
			}
			sort.Strings(leaks)
			if len(leaks) > 0 {
				r.add("PATH_HYGIENE", "FAIL", strings.Join(leaks, "; "))
			} else {
				r.add("PATH_HYGIENE", "PASS", "no machine-local paths or personal emails in the record files")
			}
		}

		// VERSION_CONSISTENCY (v0.2.0): the verdict_id suffix and the
		// record-version statement in limitations.md must tell the same story.
		{
			m := regexp.MustCompile(`^V-[A-Z]+-([0-9]+)$`).FindStringSubmatch(strings.TrimSpace(ver.VerdictID))
			if m == nil {
				r.add("VERSION_CONSISTENCY", "SKIP", fmt.Sprintf("verdict_id %q carries no version suffix", ver.VerdictID))
			} else {
				n := strings.TrimLeft(m[1], "0")
				if n == "" {
					n = "0"
				}
				want := "record version " + n
				lim, err := os.ReadFile(filepath.Join(trustDir, "limitations.md"))
				if err != nil {
					r.add("VERSION_CONSISTENCY", "FAIL", "limitations.md unreadable: "+err.Error())
				} else if strings.Contains(strings.ToLower(string(lim)), want) {
					r.add("VERSION_CONSISTENCY", "PASS", fmt.Sprintf("limitations.md states %q, matching verdict_id %q", want, ver.VerdictID))
				} else {
					r.add("VERSION_CONSISTENCY", "FAIL", fmt.Sprintf("verdict_id %q implies %q but limitations.md does not state it", ver.VerdictID, want))
				}
			}
		}

		for _, c := range ver.Claims {
			rep.Claims = append(rep.Claims, ClaimRef{
				ID: c.ID, Status: c.Status, Verification: c.Verification, Control: c.Control,
			})
		}
	}

	// 9. criteria-lock commit integrity (incident I-001 lesson)
	if isGitRepo(abs) {
		rep.CheckedCommit, _ = gitOut(abs, "rev-parse", "HEAD")
	}
	if isGitRepo(abs) && jsonOk {
		lock := man.CriteriaLock.Commit
		if !objectExists(abs, lock) {
			// F-24 lesson: distinguish a dangling pointer (history rewritten
			// and old objects pruned) from a lock that sits off-branch.
			r.add("LOCK_BLOB", "FAIL", "criteria-lock commit object not found in this repository (dangling pointer: the branch was rewritten and the old objects pruned?)")
			r.add("LOCK_ANCESTOR", "FAIL", "criteria-lock commit object not found in this repository (ancestry unverifiable)")
			r.add("LOCK_FIDELITY", "FAIL", "criteria-lock commit object not found in this repository (claims.md fidelity unverifiable)")
		} else {
			out, err := gitShowBytes(abs, lock, "trust/claims.md")
			if err != nil {
				r.add("LOCK_BLOB", "FAIL", "criteria-lock commit unreadable: "+short(err.Error()))
			} else if len(bytes.TrimSpace(out)) == 0 {
				r.add("LOCK_BLOB", "FAIL",
					"criteria-lock commit contains an EMPTY trust/claims.md — incident I-001 pattern; the lock is invalid")
			} else {
				r.add("LOCK_BLOB", "PASS",
					fmt.Sprintf("locked trust/claims.md non-empty (%d bytes at %s...)", len(out), shortHash(lock)))
			}
			if err := gitAncestor(abs, lock); err != nil {
				r.add("LOCK_ANCESTOR", "FAIL", "criteria-lock commit is not an ancestor of HEAD (lock from a side branch?)")
			} else {
				r.add("LOCK_ANCESTOR", "PASS", shortHash(lock)+"... is an ancestor of HEAD")
			}
			// LOCK_FIDELITY (v0.2.0): the criteria text the reader sees must
			// still be the locked text. A post-lock edit of claims.md is a
			// post-hoc criteria change: new record version + full re-run.
			onDisk, err := os.ReadFile(filepath.Join(trustDir, "claims.md"))
			switch {
			case err != nil:
				r.add("LOCK_FIDELITY", "FAIL", "working-tree claims.md unreadable: "+err.Error())
			case bytes.Equal(onDisk, out):
				r.add("LOCK_FIDELITY", "PASS", fmt.Sprintf("trust/claims.md is byte-identical to the criteria-lock commit (%s...)", shortHash(lock)))
			default:
				r.add("LOCK_FIDELITY", "FAIL", "trust/claims.md differs from the criteria-lock commit: criteria text changed after the lock — a new record version marked post-hoc with a full re-run is required")
			}
		}
		// OBJECT_REACHABLE (v0.2.0): the pinned object commit must exist in
		// this repository — rewritten-and-pruned history leaves records whose
		// verification chain can no longer be walked (v4 lesson).
		obj := strings.TrimSpace(man.Object.Commit)
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(obj) {
			r.add("OBJECT_REACHABLE", "SKIP", "object.commit is not a raw 40-hex commit hash")
		} else if !objectExists(abs, obj) {
			r.add("OBJECT_REACHABLE", "FAIL", "object.commit not found in this repository (dangling pointer: the branch was rewritten and the old objects pruned?)")
		} else {
			r.add("OBJECT_REACHABLE", "PASS", shortHash(obj)+"... is present in this repository")
		}
	} else {
		r.add("LOCK_BLOB", "SKIP", "not a git checkout or manifest unreadable")
		r.add("LOCK_ANCESTOR", "SKIP", "not a git checkout or manifest unreadable")
		r.add("LOCK_FIDELITY", "SKIP", "not a git checkout or manifest unreadable")
		r.add("OBJECT_REACHABLE", "SKIP", "not a git checkout or manifest unreadable")
	}

	rep.Checks = r.checks
	rep.Overall = "PASS"
	for _, c := range rep.Checks {
		if c.Status == "FAIL" {
			rep.Overall = "FAIL"
			break
		}
	}
	return rep, nil
}

func countPASS(claims []claimFile) int {
	n := 0
	for _, c := range claims {
		if c.Status == "PASS" {
			n++
		}
	}
	return n
}

// isGitRepo checks that root itself is a git work tree (direct .git entry:
// a directory for normal clones, a file for worktrees/submodules). It
// deliberately does NOT walk up: a validator must never inherit a parent
// repository's git context (tmp dirs nested inside unrelated checkouts).
func isGitRepo(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil
}

func git(r *exec.Cmd) (string, error) {
	var out, errb bytes.Buffer
	r.Stdout = &out
	r.Stderr = &errb
	if err := r.Run(); err != nil {
		if errb.Len() > 0 {
			return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(errb.String()))
		}
		return "", err
	}
	return out.String(), nil
}

func gitShowBytes(root, commit, path string) ([]byte, error) {
	cmd := exec.Command("git", "-C", root, "show", commit+":"+path)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func gitAncestor(root, commit string) error {
	return exec.Command("git", "-C", root, "merge-base", "--is-ancestor", commit, "HEAD").Run()
}

func shortHash(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 12 {
		s = s[:12]
	}
	return s
}

func short(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}

// ---- v0.2.0: generic contracts over run-output audit reports ---------------

// reasonCodes is the frozen CV0-TrustLayer-v3.5 vocabulary (protocol hard
// rules; spec text is frozen for the cycle).
var reasonCodes = map[string]bool{
	"DATA": true, "GRAMMAR": true, "NOISE": true, "DEPTH": true, "ACCESS": true,
	"BUDGET": true, "ENVIRONMENT": true, "CONFLICT_EVIDENCE": true, "OUT_OF_SCOPE": true,
}

var auditStatuses = map[string]bool{"ok": true, "fail": true, "inconclusive": true, "skipped": true}

type auditEntry struct {
	Name       *string                `json:"name"`
	Status     *string                `json:"status"`
	ReasonCode *string                `json:"reason_code"`
	Detail     *string                `json:"detail"`
	Metrics    map[string]interface{} `json:"metrics"`
}

type auditReportFile struct {
	CaseID  string `json:"case_id"`
	Verdict *struct {
		Status     string       `json:"status"`
		ReasonCode string       `json:"reason_code"`
		Audits     []auditEntry `json:"audits"`
	} `json:"verdict"`
}

// auditReport is a run-output evidence item that embeds a case verdict with
// the audit battery's per-audit outcomes (e.g. the per-case JSON files of a
// track-record run).
type auditReport struct {
	evidenceID string
	path       string
	caseID     string
	status     string
	reasonCode string
	audits     []auditEntry
}

func str(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func collectAuditReports(abs string, man *manifestFile) []auditReport {
	var reports []auditReport
	for _, ev := range man.Evidence {
		if !strings.HasSuffix(ev.Path, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(abs, filepath.FromSlash(ev.Path)))
		if err != nil {
			continue
		}
		var parsed auditReportFile
		if json.Unmarshal(data, &parsed) != nil || parsed.Verdict == nil {
			continue
		}
		reports = append(reports, auditReport{
			evidenceID: ev.ID,
			path:       ev.Path,
			caseID:     parsed.CaseID,
			status:     parsed.Verdict.Status,
			reasonCode: parsed.Verdict.ReasonCode,
			audits:     parsed.Verdict.Audits,
		})
	}
	return reports
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// auditPromise is the frozen machine-readable PRE-RUN promise line of
// claims.md (it must exist before the criteria lock, because claims.md is
// locked — a promise recorded only in post-hoc verdict fields is not a
// promise):
//
//	Promise: case=<case_id> audits=<audit_name[,audit_name...]> evidence=<E-ID>
//
// Meaning: in the referenced run-output audit report, the named audits must
// end with status "fail" for the recorded case.
type auditPromise struct {
	line     int
	caseID   string
	evidence string
	audits   []string
}

func parsePromises(claimsMD string) ([]auditPromise, []string) {
	var out []auditPromise
	var problems []string
	for i, line := range strings.Split(claimsMD, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "Promise:") {
			continue
		}
		malformed := func(msg string) {
			problems = append(problems, fmt.Sprintf("claims.md:%d: malformed Promise line: %s", i+1, msg))
		}
		rest := strings.TrimSpace(strings.TrimPrefix(t, "Promise:"))
		if rest == "" {
			malformed("empty")
			continue
		}
		fields := map[string]string{}
		var lineErrs []string
		for _, tok := range strings.Fields(rest) {
			kv := strings.SplitN(tok, "=", 2)
			if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
				lineErrs = append(lineErrs, fmt.Sprintf("token %q is not key=value", tok))
				continue
			}
			if _, dup := fields[kv[0]]; dup {
				lineErrs = append(lineErrs, fmt.Sprintf("duplicate key %q", kv[0]))
				continue
			}
			fields[kv[0]] = kv[1]
		}
		for _, req := range []string{"case", "audits", "evidence"} {
			if _, ok := fields[req]; !ok {
				lineErrs = append(lineErrs, fmt.Sprintf("missing %q", req))
			}
		}
		for k := range fields {
			if k != "case" && k != "audits" && k != "evidence" {
				lineErrs = append(lineErrs, fmt.Sprintf("unknown key %q", k))
			}
		}
		if len(lineErrs) > 0 {
			malformed(strings.Join(lineErrs, "; "))
			continue
		}
		var audits []string
		for _, a := range strings.Split(fields["audits"], ",") {
			if a = strings.TrimSpace(a); a != "" {
				audits = append(audits, a)
			}
		}
		if len(audits) == 0 {
			malformed("audits list is empty")
			continue
		}
		out = append(out, auditPromise{
			line: i + 1, caseID: fields["case"], evidence: fields["evidence"], audits: audits,
		})
	}
	return out, problems
}

var verdictIDRe = regexp.MustCompile(`V-[A-Z]+-[0-9]+`)
var verdictIDRangeRe = regexp.MustCompile(`(V-[A-Z]+)-([0-9]+)\.\.([0-9]+)`)
var supersedesSpanRe = regexp.MustCompile(`(?i)supersed(e[sd])?\b[^.\n]*`)

// externalRefsAllowed collects the verdict-like ids a record may legitimately
// cite: its own verdict id, the supersedes chain, explicit id ranges
// (V-X-001..004) and the manifest's optional external_references whitelist.
// Anything else reading like a record reference is a dangling citation
// (v4 lesson: V-PTM-001 was cited as if it backed a claim; it did not).
func externalRefsAllowed(verdictID string, man *manifestFile, texts map[string]string) map[string]bool {
	allowed := map[string]bool{}
	if verdictID != "" {
		allowed[verdictID] = true
	}
	for _, x := range man.ExternalReferences {
		if id := strings.TrimSpace(x); id != "" {
			allowed[id] = true
		}
	}
	for _, text := range texts {
		for _, span := range supersedesSpanRe.FindAllString(text, -1) {
			for _, id := range verdictIDRe.FindAllString(span, -1) {
				allowed[id] = true
			}
		}
		for _, m := range verdictIDRangeRe.FindAllStringSubmatch(text, -1) {
			lo, errL := strconv.Atoi(m[2])
			hi, errH := strconv.Atoi(m[3])
			if errL != nil || errH != nil || hi < lo || hi-lo > 1000 {
				continue
			}
			width := len(m[2])
			for n := lo; n <= hi; n++ {
				allowed[fmt.Sprintf("%s-%0*d", m[1], width, n)] = true
			}
		}
	}
	return allowed
}

func gitOut(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func objectExists(root, commit string) bool {
	if commit == "" {
		return false
	}
	return exec.Command("git", "-C", root, "cat-file", "-e", commit+"^{commit}").Run() == nil
}
