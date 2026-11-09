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

## Checking a record

```bash
tc -root /path/to/repo          # report: trust_check_report.json, exit 0/1
```

Checks: FILES_PRESENT, JSON_VALID, HASHES_MATCH, CLAIMS_XREF, EVIDENCE_XREF,
CONTROLS, REASON_CODES, VALID_UNTIL, LOCK_BLOB, LOCK_ANCESTOR.

`LOCK_BLOB` is the direct lesson of incident I-001: a criteria-lock commit
must contain a NON-EMPTY `trust/claims.md`; an empty lock is an invalid
lock. `LOCK_ANCESTOR` requires the lock commit to be an ancestor of HEAD.
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
