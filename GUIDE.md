# GUIDE — using the CV→0 Trust Layer

Detailed instructions for the three roles the protocol was designed for:
the **author** of a record, the **verifier** of someone else's record, and
the **machine** (agents, CI, scripts). Read the
[specification](specs/trust-layer-v3.5.md) once before the first record.

---

## 0. The model in one page

A Trust Record is four files in a repository:

```
trust/claims.md              atomic claims with PASS/FAIL criteria, frozen BEFORE runs
trust/evidence-manifest.json artifacts + sha256 + provenance roles + disclosure
trust/verdict.json           status per claim, procedures, valid_until
trust/limitations.md         what was NOT verified, assumptions, revision conditions
```

Chain of trust: `Claim → Evidence → Provenance → Verification → Verdict →
Limitations`. Four properties stay separate: **integrity** (hash) ≠
**authenticity** (provenance) ≠ **reproducibility** (re-run) ≠ **validity**
(the method supports the conclusion). A hash proves nothing about truth; a
re-run proves nothing about correctness. Hence: no Trust Score.

The division of labor is absolute: **machines verify mechanics, humans sign
verdicts**. A green auto-check never raises a claim's status; a signature
never exists without a green auto-check.

---

## 1. Install `tc` (the mechanical validator)

### Option A — prebuilt binary (no Go needed)

```bash
# download from GitHub Releases of the protocol repo (or from the object repo)
curl -fL -o tc https://github.com/sensus-stoa/cv0-trust-protocol/releases/download/v3.6.0/tc-v0.1.0-linux-amd64
sha256sum tc                                     # compare with releases/tc-v0.1.0-linux-amd64.sha256
chmod +x tc && mv tc ~/.local/bin/tc             # ~/.local/bin must be in PATH
```

The binary is a static stripped ELF (no libc dependency): it runs on any
glibc/musl Linux and survives `scp` to another machine.

**Primary reproduction of release v3.6.0** (preferred over downloading the
binary): `git clone https://github.com/sensus-stoa/cv0-trust-protocol &&
git checkout v3.6.0` — then the build contract in "Option B" reproduces
`releases/tc-v0.1.0-linux-amd64` byte-for-byte. The Zenodo deposit
(doi:10.5281/zenodo.24102625) archives the same snapshot — see
"Archived deposit variant" below.

### Option B — build from the vendored source

```bash
cd cv0-trust-protocol/checker        # or <object-repo>/scripts/trust_check
./install.sh                         # system Go
GO=$HOME/sdk/go1.27.3/bin/go ./install.sh   # pinned Go when system Go lags
```

`install.sh` builds a stripped static binary into `bin/tc` and deploys a
real (non-symlink) copy to `~/.local/bin/tc`; both copies are
sha256-identical. Rebuild + reinstall after any change.

### Name collision note

The acronym `tc` is free in a user PATH: iproute2's traffic-control `tc`
lives in `/usr/sbin`, which is absent from user PATH. Under root, use the
full path `/usr/sbin/tc` for traffic control and `tc` for this tool, or
override with `TC_INSTALL_DIR`.

---

## 2. Author flow — create a Trust Record for your object

Work inside the object's repository (code repo), directory `trust/`.

### Step 1 — freeze criteria (BEFORE experiments)

Write `trust/claims.md`: one heading per claim (`## C-01 …`), each with an
exact formulation, a scope, and a PASS criterion sharp enough that a
stranger can decide it. Then commit — this commit IS the criteria-lock:

```bash
git add trust/claims.md
git commit -m "criteria-lock: claims C-01..C-03 fixed before any verification"
```

Rule: changing criteria afterwards is a NEW version marked `post-hoc`, and
`PASS` on it requires new runs. The lock is valid only if the lock commit
contains a **non-empty** claims.md and is an ancestor of HEAD (the checker
enforces both; see incident P-001).

### Step 2 — collect evidence and manifest

List every artifact in `trust/evidence-manifest.json`: id (`E-…`), type,
path or URL, `sha256` for checkable files (null for regenerable outputs —
explain why in `note`), the provenance `role`
(`author-generated`/`author-executed`/`author-reported`/`client-generated`/
`client-executed`/`provider-verified`/`independently-reproduced`), and
`disclosure` (`public`/`client`/`sealed`).

Roles are corners you must not round: `provider-verified` is not
`independently reproduced`; author self-verification is allowed but must
say so (see the worked example's `verifier` field).

### Step 3 — negative controls for reliability claims

Any claim whose PASS rests on statistics (false-positive rates, calibration,
"no spurious results") needs a recorded negative control in the manifest:
id (`NC-…`), N ≥ 1, and the false-acceptance count/rate. "0 false" without
N is not accepted — for 0 of N report ≈ 3/N (rule of three, 95%).

### Step 4 — verdict

After runs, write `trust/verdict.json`: per claim — status
(`PASS`/`FAIL`/`INCONCLUSIVE`), the verification procedure (`V-…`), evidence
ids, the negative control id where applicable, and for FAIL/INCONCLUSIVE a
`reason_code` (scientific: `DATA`, `GRAMMAR`, `NOISE`, `DEPTH`; operational:
`ACCESS`, `BUDGET`, `ENVIRONMENT`, `CONFLICT_EVIDENCE`, `OUT_OF_SCOPE`).
Scientific and operational codes must not be confused. Always set
`valid_until` (a date, or `object-change`).

### Step 5 — limitations

`trust/limitations.md`: what was NOT tested, assumptions, known failure
modes, disclosure of AI assistance, and the conditions under which the
verdict will be revised. This file is what keeps the record honest.

### Step 6 — machine check

```bash
tc -root .
```

Exit 0 + "overall: PASS" — the mechanics hold. A FAIL here is a defect in
the record's mechanics: fix the record (never the criteria after the lock)
and re-run.

### Step 7 — human sign-off (the verdict layer)

```bash
tc -root . --signoff --signer "Your Name"
```

Only runs on a green report. For each claim: `Space/Enter` = confirm, `s` =
skip, `q` = abort. Result: `trust/signoff.md` with date, signer, confirmed
claims, and the sha256 of the report the signature applies to. Commit it:

```bash
git add trust/signoff.md
git commit -m "trust: human sign-off of <verdict-id>"
git push
```

The signature binds to the report hash: any change to the record or the
object invalidates it and requires a fresh auto-check plus a fresh
signature. Agents never sign — that is the point of the layer.

---

## 3. Verifier flow — independently verify someone else's record

1. Clone the repository at the pinned version (the `object.commit` in the
   manifest, never HEAD).
2. Run `tc -root .` — the mechanical gate (locks, hashes, cross-references)
   must be green before anything else matters.
3. Walk the chain by hand: claim text → evidence rows → provenance roles →
   verification procedures → verdict → limitations. The spec's quality
   criterion: a third-party engineer must be able to trace everything.
4. Reproduce the procedures (`V-…`) in **your** environment; treat your
   results as your own verdict, not an amendment of theirs.
5. If you publish your verification: your OWN `verdict_id` against the SAME
   criteria-lock and the same `object_version` — never an edit of the
   author's verdict.json. Mark your role honestly
   (`independently-reproduced` requires an environment separated from the
   original run).

---

## 4. Machine flow — agents, CI, scripts

- Stable schema URLs (JSON Schema draft 2020-12):
  - `https://raw.githubusercontent.com/sensus-stoa/cv0-trust-protocol/main/schemas/evidence-manifest.schema.json`
  - `https://raw.githubusercontent.com/sensus-stoa/cv0-trust-protocol/main/schemas/verdict.schema.json`
- Validate a record without cloning the object (any 2020-12 validator):

```python
import json, jsonschema
schema = json.load(open("verdict.schema.json"))
record = json.load(open("trust/verdict.json"))
jsonschema.Draft202012Validator(schema).validate(record)
```

- `tc` exit codes are a CI contract: `0` = mechanics green, `1` = mechanics
  broken, `2` = tooling error. A record that fails `tc` must not be quoted,
  signed, or promoted.
- The report (`trust_check_report.json`) carries every check with details;
  `--signoff` records its sha256, so a signature is meaningless without the
  exact report it names.

---

## 5. Check the checker (Verifier Track Record)

A validator that always says PASS is useless. `checks_test.go` plants
defects and asserts the checker calls them (N=5): clean record → PASS;
tampered sha256 → FAIL; INCONCLUSIVE without reason_code → FAIL;
statistical PASS without control → FAIL; empty criteria-lock blob (incident
I-001) → FAIL.

```bash
cd checker && go test -v .
```

Results are recorded with N in the object's
`docs/verifier-track-record.md`. Rule: any change to `checks.go` re-runs
all five cases before commit.

---

## 6. Incident flow

- **Object-level incidents** (a broken record in some repo) live in that
  repo's `trust/trust-incidents.md`: symptom, cause, action, evidence of
  criteria immutability (empty diff or hash), status, lesson.
- **Protocol-level incidents** (a defect or gap in the SPEC itself) live in
  this repository's [`trust-incidents.md`](trust-incidents.md). The change
  rule: spec changes happen only in response to such an entry.

Restoring a lost record from a valid lock: recovery of the same text is
byte-for-byte, is not a criteria change, and does not require re-runs;
different text is a new post-hoc version and does require them.

---

## Archived deposit variant (Zenodo)

The Zenodo deposit (doi:10.5281/zenodo.24102625, edition v3.6.0) contains
the same files as this repository at tag `v3.6.0` — packaged as
`cv0-trust-layer-v3.6.0.tar.gz` plus a per-file hash list
(`archive-file-hashes.txt`, an external sidecar of the packaging, not part
of the tar). Verification without git:

```bash
sha256sum cv0-trust-layer-v3.6.0.tar.gz          # == d9c4f5a0…3544
tar -xzf cv0-trust-layer-v3.6.0.tar.gz
cd cv0-trust-layer-v3.6.0/checker
go test -count=1 .                                # 5/5 self-tests
go build -trimpath -buildvcs=false -ldflags "-s -w" -o tc .
sha256sum tc                                      # == releases/tc-v0.1.0-linux-amd64.sha256

# per-file provenance against the author's external sidecar:
sha256sum $(find cv0-trust-layer-v3.6.0 -type f) > my-hashes.txt
diff <(sort -k2 my-hashes.txt) <(sort -k2 archive-file-hashes.txt)   # empty = identical
```

Release editions and the spec text version are separate axes: the deposit
is release edition v3.6.0 carrying specification text v3.5. See the
repository README §"Versioning".

---

## 7. Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `LOCK_BLOB FAIL: criteria-lock commit contains an EMPTY trust/claims.md` | the lock commit captured an empty file (the I-001 pattern) | re-lock with the identical criteria text (or recover it), point `criteria_lock.commit` at the new commit; re-runs not required if the text is unchanged |
| `HASHES_MATCH FAIL: declared …, actual …` | an artifact changed after the manifest was written | deliberate change → new record cycle (supersede/update evidence deliberately); accidental → restore the artifact |
| `CONTROLS FAIL: statistical PASS … without negative control` | a PASS claim leans on statistics with no recorded control | add the control with N, or downgrade the claim to INCONCLUSIVE/OUT_OF_SCOPE |
| Git output looks contradictory or impossible | a git wrapper/alias in the environment may fabricate output | re-diagnose with the unaliased binary: `command git`; cross-check via `git ls-tree` / raw file URLs |
| `--signoff` refuses to start | the auto-check is not green | run `tc -root .`, fix the FAILs; this is the rule working, not a bug |
| `tc` builds but tests reference `../../trust` | vendored inside an object repo | expected: `TestLiveRepoRecordPasses` skips when the object has no record; fixture tests are self-contained in `testdata/` |

---

## 8. FAQ

**Does a green `tc` mean the claims are true?**
No. It means the mechanics hold: files exist, hashes match, references
resolve, controls exist, the lock is valid. The meaning of a PASS is bound
to the frozen criteria text and argued by humans; the sign-off is the human
layer of exactly that.

**Why no Trust Score?**
A single number hides different kinds of uncertainty (hash-mismatch vs
missing control vs conflicting evidence) and invites marketing. Statuses
carry the uncertainty explicitly.

**Can an agent sign?**
An agent can run everything machine-side, including preparing reports and
harnesses. The sign-off keypress belongs to a human with a name; the
protocol records `executor` and `verifier` separately for exactly this
reason.

**Where do independent verifications live?**
As their own `verdict_id` entries (new files or a new verdict record in the
object repo) against the same criteria-lock and the same object_version —
never as edits inside the author's verdict.json. See limitations of the
worked example.
