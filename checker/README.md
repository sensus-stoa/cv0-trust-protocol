# tc — trust_check (mechanical validator)

The mechanical validator of a CV→0 Trust Record (Trust Layer v3.5). One Go
binary, stdlib-only, zero dependencies.

Principle: **the machine checks mechanics, the human signs the meaning**.
An auto-PASS never raises a claim's status — it states only that the
mechanics hold.

## Build and install

```bash
./install.sh
# on a non-default Go toolchain:
GO=$HOME/sdk/go1.27.3/bin/go ./install.sh
```

The installer builds a **static stripped binary** `bin/tc` and deploys a
real file (not a symlink) to `~/.local/bin/tc`; `bin/tc` and the deployed
copy are sha256-identical. Upgrade after edits: run `./install.sh` again.
The binary has no libc dependency and can be `scp`ed anywhere as-is.

Build contract: `-trimpath -buildvcs=false -ldflags "-s -w"` — a rebuild
from the same commit reproduces the shipped `releases/tc-*` binary
byte-for-byte. Primary reproduction of a release edition:
`git checkout <release-tag>` then the build above.

## Checking a record

```bash
tc -root /path/to/repo          # report: trust_check_report.json, exit 0/1
```

Record integrity (files, hashes, cross-references, lock):

- FILES_PRESENT, JSON_VALID, HASHES_MATCH (evidence + disclosures),
  CLAIMS_XREF, EVIDENCE_XREF, CONTROLS, REASON_CODES, VALID_UNTIL
- LOCK_BLOB, LOCK_ANCESTOR, LOCK_FIDELITY, OBJECT_REACHABLE

`LOCK_BLOB` is the direct lesson of incident I-001: a criteria-lock commit
must contain a NON-EMPTY `trust/claims.md`; an empty lock is an invalid
lock. `LOCK_ANCESTOR` requires the lock commit to be an ancestor of HEAD.
`LOCK_FIDELITY` (v0.2.0) requires the working-tree `trust/claims.md` to
remain BYTE-IDENTICAL to the blob at the criteria-lock commit: a post-lock
criteria edit is a post-hoc change and demands a new record version with a
full re-run. `OBJECT_REACHABLE` (v0.2.0) requires the manifest's
`object.commit` to exist in the repository; LOCK_BLOB/LOCK_ANCESTOR/
LOCK_FIDELITY distinguish a dangling pointer (history rewritten and old
objects pruned) from an off-branch lock — history rewrites after
publication leave old record snapshots unverifiable by design, so do the
author/email hygiene BEFORE the first commit and never rewrite a published
lock.

Record content (v0.2.0, all domain-agnostic — project specifics are
declared as data in the manifest, never hardcoded in tc):

- AUDIT_SHAPE: any evidence item that embeds a case verdict with the audit
  battery's outcomes must satisfy the serialization contract (statuses from
  frozen enums; ok-entries carry reason_code "" plus a string detail and an
  object metrics; non-ok entries cite the frozen reason-code vocabulary)
- AUDIT_DECISION_XREF: optional structured decision metadata on verdict
  claims (case_verdict, case_reason_code, failing_audits,
  inconclusive_audits — all four together or none) must match the
  referenced run-output report exactly; inconclusive audits may not be
  silently dropped
- CLAIM_PROMISE: pre-run audit promises are declared in the LOCKED
  claims.md as marker lines —
  `Promise: case=<case_id> audits=<a1,a2,...> evidence=<E-ID>` —
  and must hold in the referenced report (every promised audit ends with
  status "fail"). A promise recorded only in post-hoc verdict fields is
  not a promise; a record without Promise lines SKIPs the check
- REQUIRED_METRICS: manifest-declared contract
  `audit_metrics_contract: {<audit>: {on_status, required_keys}}` — when
  the named audit ends in on_status, the report metrics must carry the
  required keys
- EVIDENCE_SECTION: content that is NOT evidence (pre-lock calibration,
  drafts) must live in a `disclosures` section, never reuse an evidence id
- EXTERNAL_REF_WHITELIST: verdict-like ids cited by the record must resolve
  within it (own verdict id, the supersedes chain, explicit ranges
  V-X-001..004) or be declared in `manifest.external_references`
- PATH_HYGIENE: no machine-local paths (/home/…, /Users/…) or personal
  emails in the record files
- VERSION_CONSISTENCY: the verdict_id suffix and the record-version
  statement in limitations.md must agree

Checks absent from a record are SKIPped, never silently dropped: the report
shows which contracts a record opts into.

The report carries `checked_commit` (HEAD of the checked work tree) and
records `root` exactly as passed on the command line — run `tc -root .`
from the repository root to keep the report free of machine-local paths.
The validator never walks up the directory tree to discover a git context.

## Human sign-off (the verdict layer)

```bash
tc -root . --signoff --signer "Your Name"
```

Refuses to start without a green auto-check; per claim:
`Space/Enter = confirm, s = skip, q = abort`. Result: `trust/signoff.md`
with the signer, date, confirmed claims, and the sha256 of the report.
A green auto-check without `signoff.md` is not a verdict; any change to the
record or the object invalidates the signature and requires a fresh run.

## Self-test (negative controls)

```bash
go test -v .
```

N=5 fixture-based cases: tampered sha256 → FAIL; INCONCLUSIVE without
reason_code → FAIL; a statistical PASS without a control → FAIL; a clean
record → PASS; and the I-001 regression (an empty blob in a lock commit →
FAIL). The fixture record lives in `testdata/`, so the vendored checker is
self-contained in any repository. Live-repo coverage activates
automatically when the checker sits next to a real `trust/` record
(`TestLiveRepoRecordPasses`). Note: the checker never walks up the
directory tree to find a git context — lock checks apply only when the
validated root itself is a git work tree.

Full protocol: the parent repository's [GUIDE](../GUIDE.md) and
[specification](../specs/trust-layer-v3.5.md).
