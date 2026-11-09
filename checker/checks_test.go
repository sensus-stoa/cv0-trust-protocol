package main

import (
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
