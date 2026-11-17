package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The vendored checker is self-sufficient: fixture records live in
// checker/testdata/, so the tests run in any repository that carries
// this directory. TestLiveRepoRecordPasses adds coverage inside object
// repositories when their real trust/ record is present.

func fixtureRoot(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dst, "trust"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"claims.md", "evidence-manifest.json", "verdict.json", "limitations.md"} {
		data, err := os.ReadFile(filepath.Join("testdata", "trust", f))
		if err != nil {
			t.Fatalf("fixture missing: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dst, "trust", f), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join("testdata", "artifact.txt"))
	if err != nil {
		t.Fatalf("fixture artifact missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "artifact.txt"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func editManifestJSON(t *testing.T, root string, fn func(m map[string]interface{})) {
	t.Helper()
	p := filepath.Join(root, "trust", "evidence-manifest.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	fn(m)
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func editVerdictJSON(t *testing.T, root string, fn func(m map[string]interface{})) {
	t.Helper()
	p := filepath.Join(root, "trust", "verdict.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	fn(m)
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(out, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func expectOverall(t *testing.T, root, want string) *Report {
	t.Helper()
	rep, err := runChecks(root)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Overall != want {
		for _, c := range rep.Checks {
			if c.Status == "FAIL" {
				t.Errorf("check %s FAILED: %s", c.ID, c.Detail)
			}
		}
		t.Fatalf("expected overall %s, got %s", want, rep.Overall)
	}
	return rep
}

func expectFailCheck(t *testing.T, rep *Report, id string) {
	t.Helper()
	for _, c := range rep.Checks {
		if c.ID == id && c.Status == "FAIL" {
			return
		}
	}
	t.Errorf("expected FAIL on check %s; got overall=%s checks=%+v", id, rep.Overall, rep.Checks)
}

// Clean fixture record must be mechanically green.
func TestFixtureRecordPasses(t *testing.T) {
	expectOverall(t, fixtureRoot(t), "PASS")
}

// Coverage fallback: when the checker is vendored inside an object repo
// (e.g. scripts/trust_check next to a real trust/), the live record is
// exercised too. Skipped in repositories without a trust/ record.
func TestLiveRepoRecordPasses(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "trust", "verdict.json")); err != nil {
		t.Skip("no trust/ record in this repository")
	}
	expectOverall(t, root, "PASS")
}

// Negative control 1: a tampered artifact hash must break the report.
func TestBrokenHashFails(t *testing.T) {
	root := fixtureRoot(t)
	editManifestJSON(t, root, func(m map[string]interface{}) {
		ev, _ := m["evidence"].([]interface{})
		first, _ := ev[0].(map[string]interface{})
		if h, ok := first["sha256"].(string); ok && len(h) > 0 {
			flip := "0"
			if h[0] == '0' {
				flip = "1"
			}
			first["sha256"] = flip + h[1:]
		}
	})
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "HASHES_MATCH")
}

// Negative control 2: INCONCLUSIVE without reason_code must break the report.
func TestMissingReasonCodeFails(t *testing.T) {
	root := fixtureRoot(t)
	editVerdictJSON(t, root, func(m map[string]interface{}) {
		claims, _ := m["claims"].([]interface{})
		for _, c := range claims {
			cm, _ := c.(map[string]interface{})
			if cm["status"] == "INCONCLUSIVE" {
				delete(cm, "reason_code")
			}
		}
	})
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "REASON_CODES")
}

// Negative control 3: statistical PASS (verification references V-02)
// without a negative control reference must break the report.
func TestPASSWithoutControlFails(t *testing.T) {
	root := fixtureRoot(t)
	editVerdictJSON(t, root, func(m map[string]interface{}) {
		claims, _ := m["claims"].([]interface{})
		for _, c := range claims {
			cm, _ := c.(map[string]interface{})
			if cm["control"] == "NC-01" {
				delete(cm, "control")
			}
		}
	})
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "CONTROLS")
}

// I-001 regression: a criteria-lock commit whose claims.md is an empty
// blob makes the whole record FAIL (the GitHub-display incident pattern).
func TestLockWithEmptyBlobFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "trust"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")

	if err := os.WriteFile(filepath.Join(dir, "trust", "claims.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	run("-c", "user.email=fixture@local", "-c", "user.name=fixtures", "add", "-A")
	run("-c", "user.email=fixture@local", "-c", "user.name=fixtures", "commit", "-q", "-m", "lock (empty claims: the I-001 defect)")
	lockCommit := run("rev-parse", "HEAD")

	ver := map[string]interface{}{
		"verdict_id": "V-FIXTURE", "date": "2026-11-09", "valid_until": "object-change",
		"object_version": "abcdef1234567890abcdef1234567890abcdef12",
		"claims": []map[string]interface{}{{
			"id": "C-01", "status": "PASS", "verification": "V-01: clean run", "evidence": []string{"E-01"},
		}},
	}
	man := map[string]interface{}{
		"schema": "trust-manifest/v3.5", "date": "2026-11-09",
		"criteria_lock": map[string]interface{}{"commit": lockCommit, "date": "2026-11-09", "note": "empty-blob lock fixture"},
		"evidence": []map[string]interface{}{{
			"id": "E-01", "type": "code", "path": "trust/claims.md", "sha256": nil,
		}},
		"controls": []map[string]interface{}{{"id": "NC-FIXTURE", "N": 50}},
	}
	writeJSON := func(name string, v interface{}) {
		b, _ := json.MarshalIndent(v, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "trust", name), append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON("evidence-manifest.json", man)
	writeJSON("verdict.json", ver)
	if err := os.WriteFile(filepath.Join(dir, "trust", "limitations.md"), []byte("# fixture"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp := expectOverall(t, dir, "FAIL")
	expectFailCheck(t, resp, "LOCK_BLOB")
}

// ---- v0.2.0 checks ---------------------------------------------------------

// writeAuditReport writes a run-output audit report into the fixture root
// and registers it as hashed evidence, so only the check under test fires.
func writeAuditReport(t *testing.T, root, name, evidenceID string, verdict map[string]interface{}) {
	t.Helper()
	body := map[string]interface{}{"case_id": "case-a", "verdict": verdict}
	b, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fileBytes := append(b, '\n')
	if err := os.WriteFile(filepath.Join(root, name), fileBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(fileBytes)
	editManifestJSON(t, root, func(m map[string]interface{}) {
		ev, _ := m["evidence"].([]interface{})
		entry := map[string]interface{}{
			"id": evidenceID, "type": "run-output", "path": name,
			"sha256": hex.EncodeToString(sum[:]),
		}
		// upsert: a rewritten report must replace its earlier hash, not
		// append a second entry with the same id
		for i, it := range ev {
			if itMap, _ := it.(map[string]interface{}); itMap != nil && itMap["id"] == evidenceID {
				ev[i] = entry
				return
			}
		}
		m["evidence"] = append(ev, entry)
	})
}

// goodAuditVerdict is a well-shaped FAIL report: frozen reason codes,
// ok-entries with empty reason_code, string details, object metrics.
func goodAuditVerdict() map[string]interface{} {
	return map[string]interface{}{
		"status":      "FAIL",
		"reason_code": "NOISE",
		"audits": []map[string]interface{}{
			{"name": "claim_conditions", "status": "fail", "reason_code": "NOISE", "detail": "d", "metrics": map[string]interface{}{"k": "v"}},
			{"name": "trial_disclosure", "status": "fail", "reason_code": "NOISE", "detail": "d", "metrics": map[string]interface{}{}},
			{"name": "metric_reproduction", "status": "ok", "reason_code": "", "detail": "d", "metrics": map[string]interface{}{}},
		},
	}
}

func TestAuditShapeCleanReportPasses(t *testing.T) {
	root := fixtureRoot(t)
	writeAuditReport(t, root, "report.json", "E-R1", goodAuditVerdict())
	expectOverall(t, root, "PASS")
}

// The planted defect that corrupted per-case evidence in an object record:
// narrative text passed positionally into reason_code of an ok-audit.
func TestAuditShapeRejectsTextInOkReasonCode(t *testing.T) {
	root := fixtureRoot(t)
	v := goodAuditVerdict()
	audits := v["audits"].([]map[string]interface{})
	audits[2]["reason_code"] = "reported MAE reproduces (narrative leaked)"
	writeAuditReport(t, root, "report.json", "E-R1", v)
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "AUDIT_SHAPE")
}

func TestAuditShapeRejectsUnknownReasonCode(t *testing.T) {
	root := fixtureRoot(t)
	v := goodAuditVerdict()
	audits := v["audits"].([]map[string]interface{})
	audits[0]["reason_code"] = "WHATEVER"
	writeAuditReport(t, root, "report.json", "E-R1", v)
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "AUDIT_SHAPE")
}

// AUDIT_DECISION_XREF: decision metadata must match the report exactly —
// a claim may not quietly disagree with its own evidence.
func TestAuditDecisionXrefMatchesAndMismatches(t *testing.T) {
	root := fixtureRoot(t)
	writeAuditReport(t, root, "report.json", "E-R1", goodAuditVerdict())
	editVerdictJSON(t, root, func(m map[string]interface{}) {
		claims, _ := m["claims"].([]interface{})
		cm, _ := claims[0].(map[string]interface{}) // C-01
		cm["case_verdict"] = "FAIL"
		cm["case_reason_code"] = "NOISE"
		cm["failing_audits"] = []string{"trial_disclosure", "claim_conditions"}
		cm["inconclusive_audits"] = []string{}
		cm["evidence"] = []string{"E-01", "E-R1"}
	})
	expectOverall(t, root, "PASS")

	editVerdictJSON(t, root, func(m map[string]interface{}) {
		claims, _ := m["claims"].([]interface{})
		cm, _ := claims[0].(map[string]interface{})
		cm["case_verdict"] = "PASS" // quietly soften the recorded verdict
	})
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "AUDIT_DECISION_XREF")
}

// CLAIM_PROMISE: promises live in the LOCKED claims.md and must hold in the
// run-output evidence; a promised audit going "ok" breaks the record.
func TestClaimPromiseViolatedFails(t *testing.T) {
	root := fixtureRoot(t)
	writeAuditReport(t, root, "report.json", "E-R1", goodAuditVerdict())
	claimsPath := filepath.Join(root, "trust", "claims.md")
	claims, err := os.ReadFile(claimsPath)
	if err != nil {
		t.Fatal(err)
	}
	claims = append(claims, []byte("\nPromise: case=case-a audits=claim_conditions,trial_disclosure evidence=E-R1\n")...)
	if err := os.WriteFile(claimsPath, claims, 0o644); err != nil {
		t.Fatal(err)
	}
	expectOverall(t, root, "PASS")

	writeAuditReport(t, root, "report.json", "E-R1", func() map[string]interface{} {
		v := goodAuditVerdict()
		audits := v["audits"].([]map[string]interface{})
		audits[1]["status"] = "ok" // the promised audit quietly passes
		audits[1]["reason_code"] = ""
		return v
	}())
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "CLAIM_PROMISE")
}

func TestClaimPromiseMalformedFails(t *testing.T) {
	root := fixtureRoot(t)
	writeAuditReport(t, root, "report.json", "E-R1", goodAuditVerdict())
	claimsPath := filepath.Join(root, "trust", "claims.md")
	claims, err := os.ReadFile(claimsPath)
	if err != nil {
		t.Fatal(err)
	}
	claims = append(claims, []byte("\nPromise: case=case-a evidence=E-R1\n")...) // audits missing
	if err := os.WriteFile(claimsPath, claims, 0o644); err != nil {
		t.Fatal(err)
	}
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "CLAIM_PROMISE")
}

// REQUIRED_METRICS: a manifest-declared contract (data, not code).
func TestRequiredMetricsContract(t *testing.T) {
	root := fixtureRoot(t)
	writeAuditReport(t, root, "report.json", "E-R1", goodAuditVerdict())
	editManifestJSON(t, root, func(m map[string]interface{}) {
		m["audit_metrics_contract"] = map[string]interface{}{
			"claim_conditions": map[string]interface{}{
				"on_status": "fail", "required_keys": []string{"relative_gap"},
			},
		}
	})
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "REQUIRED_METRICS")

	writeAuditReport(t, root, "report.json", "E-R1", func() map[string]interface{} {
		v := goodAuditVerdict()
		audits := v["audits"].([]map[string]interface{})
		audits[0]["metrics"] = map[string]interface{}{"relative_gap": 0.5}
		return v
	}())
	expectOverall(t, root, "PASS")
}

// EVIDENCE_SECTION: non-evidence content must live in disclosures[], not in
// evidence[] (an id collision is the reuse of a spent evidence slot).
func TestEvidenceSectionCollisionFails(t *testing.T) {
	root := fixtureRoot(t)
	editManifestJSON(t, root, func(m map[string]interface{}) {
		m["disclosures"] = []map[string]interface{}{{
			"id": "E-01", "path": "artifact.txt",
			"sha256": "df266822f87e2b0b8d12f9a3d742700e12cf6a74397926767206f856cfe0561e",
		}}
	})
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "EVIDENCE_SECTION")

	editManifestJSON(t, root, func(m map[string]interface{}) {
		m["disclosures"] = []map[string]interface{}{{
			"id": "D-01", "path": "artifact.txt",
			"sha256": "df266822f87e2b0b8d12f9a3d742700e12cf6a74397926767206f856cfe0561e",
		}}
	})
	expectOverall(t, root, "PASS")
}

// EXTERNAL_REF_WHITELIST: dangling external record citations fail; the
// manifest whitelist and the supersedes chain are the legitimate routes.
func TestExternalRefWhitelist(t *testing.T) {
	root := fixtureRoot(t)
	claimsPath := filepath.Join(root, "trust", "claims.md")
	claims, err := os.ReadFile(claimsPath)
	if err != nil {
		t.Fatal(err)
	}
	claims = append(claims, []byte("\nAs measured in V-FOREIGN-001 (claim C-99).\n")...)
	if err := os.WriteFile(claimsPath, claims, 0o644); err != nil {
		t.Fatal(err)
	}
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "EXTERNAL_REF_WHITELIST")

	editManifestJSON(t, root, func(m map[string]interface{}) {
		m["external_references"] = []string{"V-FOREIGN-001"}
	})
	expectOverall(t, root, "PASS")

	editVerdictJSON(t, root, func(m map[string]interface{}) {
		m["verifier"] = "Supersedes V-OTHER-009; not independent"
	})
	expectOverall(t, root, "PASS")
}

// PATH_HYGIENE: machine-local paths and personal emails must not leak.
func TestPathHygieneFailsOnMachinePaths(t *testing.T) {
	root := fixtureRoot(t)
	lim := filepath.Join(root, "trust", "limitations.md")
	b, err := os.ReadFile(lim)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(lim, append(b, []byte("\n- Raw output lived in /home/ninjacat/notes.txt.\n")...), 0o644)
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "PATH_HYGIENE")
}

// VERSION_CONSISTENCY: verdict_id suffix and the limitations.md record
// version must agree (the stale "record version 3" header class).
func TestVersionConsistencyFailsOnStaleHeader(t *testing.T) {
	root := fixtureRoot(t)
	lim := filepath.Join(root, "trust", "limitations.md")
	b, err := os.ReadFile(lim)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(lim, bytes.ReplaceAll(b, []byte("record version 1"), []byte("record version 9")), 0o644)
	rep := expectOverall(t, root, "FAIL")
	expectFailCheck(t, rep, "VERSION_CONSISTENCY")
}

// gitRecordFixture builds a real git work tree: claims frozen by a lock
// commit, then the record files committed on top.
func gitRecordFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, "trust"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, "trust", name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string) string {
		run("-c", "user.email=fixture@local", "-c", "user.name=fixtures", "add", "-A")
		run("-c", "user.email=fixture@local", "-c", "user.name=fixtures", "commit", "-q", "-m", msg)
		return run("rev-parse", "HEAD")
	}
	write("claims.md", []byte("# fixture claims\n\n## C-01 clean\n\nPASS criterion: nothing.\nVerification V-01: clean run.\n"))
	lock := commit("criteria lock (claims frozen)")
	writeJSON := func(name string, v interface{}) {
		b, _ := json.MarshalIndent(v, "", "  ")
		write(name, append(b, '\n'))
	}
	writeJSON("evidence-manifest.json", map[string]interface{}{
		"schema": "trust-manifest/v3.5", "date": "2026-11-17",
		"criteria_lock": map[string]interface{}{"commit": lock, "date": "2026-11-17", "note": "fixture lock"},
		"object":        map[string]interface{}{"commit": lock},
		"evidence": []map[string]interface{}{
			{"id": "E-01", "type": "code", "path": "trust/claims.md", "sha256": nil},
		},
		"controls": []map[string]interface{}{{"id": "NC-FIXTURE", "N": 50}},
	})
	writeJSON("verdict.json", map[string]interface{}{
		"verdict_id": "V-FIXTURE-001", "date": "2026-11-17", "valid_until": "object-change",
		"object_version": lock,
		"claims": []map[string]interface{}{
			{"id": "C-01", "status": "PASS", "verification": "V-01: clean run", "evidence": []string{"E-01"}},
		},
	})
	write("limitations.md", []byte("# fixture limitations (record version 1)\n"))
	commit("record files")
	return dir
}

// LOCK_FIDELITY: claims.md must remain byte-identical to the criteria-lock
// commit — a post-lock edit (even uncommitted) is a post-hoc criteria change.
func TestLockFidelityFailsOnPostLockEdit(t *testing.T) {
	dir := gitRecordFixture(t)
	expectOverall(t, dir, "PASS") // pre-edit state is green

	claimsPath := filepath.Join(dir, "trust", "claims.md")
	b, err := os.ReadFile(claimsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claimsPath, append(b, []byte("\nPOST-HOC EDIT\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := expectOverall(t, dir, "FAIL")
	expectFailCheck(t, rep, "LOCK_FIDELITY")
}

// F-24: dangling pointers must be diagnosed as MISSING OBJECTS, not as a
// "side branch" (the object record of v4 pointed at commits pruned by a
// history rewrite).
func TestLockPointerDanglingDiagnosed(t *testing.T) {
	dir := gitRecordFixture(t)
	editManifestJSON(t, dir, func(m map[string]interface{}) {
		cl, _ := m["criteria_lock"].(map[string]interface{})
		cl["commit"] = "857cc60c03424fdfd9f6d06b434a90f772f857b4"
		obj, _ := m["object"].(map[string]interface{})
		obj["commit"] = "23c421dd7a25de4b703fa9f3af1ea5a4e2b81190"
	})
	rep := expectOverall(t, dir, "FAIL")
	for _, id := range []string{"LOCK_BLOB", "LOCK_ANCESTOR", "LOCK_FIDELITY", "OBJECT_REACHABLE"} {
		expectFailCheck(t, rep, id)
	}
	for _, c := range rep.Checks {
		if (c.ID == "LOCK_BLOB" || c.ID == "OBJECT_REACHABLE") && !strings.Contains(c.Detail, "not found") {
			t.Errorf("%s detail should diagnose a missing object: %s", c.ID, c.Detail)
		}
	}
}

func TestReportCarriesCheckedCommit(t *testing.T) {
	dir := gitRecordFixture(t)
	rep := expectOverall(t, dir, "PASS")
	if rep.CheckedCommit == "" {
		t.Fatal("checked_commit missing on a git-root record")
	}
	if out, err := gitOut(dir, "rev-parse", "HEAD"); err != nil || out != rep.CheckedCommit {
		t.Fatalf("checked_commit %q != HEAD %q (%v)", rep.CheckedCommit, out, err)
	}
}
