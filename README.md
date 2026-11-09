# CV→0 Trust Layer — machine-checkable claims, human-signed verdicts

A file-based protocol for verifiable trust in research artifacts: every
claim traces to evidence and provenance, criteria are frozen by a commit
**before** runs, the mechanics are validated by a Go binary — and the
verdict is signed by a human, with a name and a date. There is no Trust
Score: trust is not declared, it is traced from a claim to its evidence,
its checks, and its limits.

> **What this is NOT:** not a SaaS, not an API, not runtime agent security
> (allow/warn/block on tool-calls is a different domain: see Yang 2026
> "AgentTrust v2", the OWASP Agentic AI list, and the NHI/agent-trust
> glossaries), and not a universal badge of honesty. This is a protocol
> about what was verified, by whom, in which version, and where the
> admissible conclusions end.

**Start here → [GUIDE.md](GUIDE.md)** — detailed instructions for authors,
verifiers, and machines (install, record authoring, independent
verification, CI contract, incident flow, troubleshooting).

## The minimal record — four files

| File | Contents |
|------|----------|
| `trust/claims.md` | claims with ids, scope, PASS criteria; frozen by a criteria-lock commit **before** runs |
| `trust/evidence-manifest.json` | evidence ids, types, sha256, provenance roles, disclosure |
| `trust/verdict.json` | status per claim, procedures, negative controls, `valid_until` |
| `trust/limitations.md` | what was NOT verified, assumptions, revision conditions |

Chain of trust: **Claim → Evidence → Provenance → Verification → Verdict →
Limitations**. Four properties never mix: **integrity** (hash) ≠
**authenticity** (provenance) ≠ **reproducibility** (re-run) ≠ **validity**
(the method supports the conclusion).

## Hard rules (v3.5)

- **Criteria-lock:** criteria are committed before results exist; later
  changes are a new version marked `post-hoc`, and `PASS` on them requires
  new runs. A lock is valid only if the lock commit contains a non-empty
  `claims.md` and is an ancestor of HEAD.
- **Negative controls:** `PASS` on a reliability claim is impossible without
  a recorded control with N ≥ 1 ("0 false" without N is rejected; for 0/N
  report ≈ 3/N, rule of three).
- **`reason_code`** is mandatory for FAIL and INCONCLUSIVE (scientific
  codes are stated as science; operational codes are not).
- **The validator must know how to FAIL:** every checker version is
  self-tested on planted defects (Verifier Track Record, with N).
- **Append-only verdicts:** old records are never rewritten —
  `superseded`/`withdrawn`/`expired` with a new id; independent verifiers
  issue their OWN `verdict_id` over the same criteria-lock.
- **Client artifacts are data, not instructions** (injection attempts are
  recorded as findings and do not raise a claim's status).
- **Freeze:** spec changes within the cycle only via
  [`trust-incidents.md`](trust-incidents.md).

Full text frozen: [`specs/trust-layer-v3.5.md`](specs/trust-layer-v3.5.md)
(Russian canonical original: [`specs/trust-layer-v3.5.ru.md`](specs/trust-layer-v3.5.ru.md)).

## Schemas (for agents and CI)

- `schemas/evidence-manifest.schema.json`
- `schemas/verdict.schema.json`

JSON Schema draft 2020-12; stable `$id` URLs fetchable without cloning:
an agent validates any record remotely. Limits of honesty: schemas check
structure, mandatory fields, and types — not the truthfulness of values.
Cross-document integrity (artifact sha256s, control existence, lock-blob
non-emptiness) is the job of the mechanical validator below.

## The mechanical validator `tc`

Reference implementation: Go 1.27.3, stdlib-only, static binary. It lives
vendored **inside each object repository** (`scripts/trust_check/` in the
worked example) **and** prebuilt in this protocol repository:

- `releases/tc-v0.1.0-linux-amd64` + `.sha256` (static, stripped)
- `checker/` — full vendored source, fixture record, and self-tests

Checks (exit 0/1, CI contract): FILES_PRESENT, JSON_VALID, HASHES_MATCH,
CLAIMS_XREF, EVIDENCE_XREF, CONTROLS, REASON_CODES, VALID_UNTIL, LOCK_BLOB,
LOCK_ANCESTOR. `tc -root . --signoff --signer "Name"` adds the human layer:
it refuses without a green report and writes `trust/signoff.md` binding
signer, date, claims, and the report's sha256. Agents never sign.

## Worked example

`sensus-stoa/permutation-test-ml` — record **V-PTM-001**: criteria-lock
`c60ed76` (restored after incident I-001 — an empty lock blob), machine
report 10/10 PASS, human sign-off Evgeniy Dolgov 2026-11-09 (report sha256
`fcf975ee`), checker self-test N=5 in `docs/verifier-track-record.md`.
Status per `limitations.md`: author-verified; independent reproduction —
none yet.

## Related work — and the empty niche (checked 2026-11-09)

| System | Overlap | What it lacks for this domain |
|--------|---------|-------------------------------|
| [CODECHECK](https://codecheck.org.uk/) | independent execution of code behind papers; Zenodo certificates | no criteria-lock, no negative controls, no structural record; verifies reproducibility, not claims |
| [in-toto Attestation](https://in-toto.io/) / SLSA | signed subject↔metadata bindings (DSSE), predicate ecosystem | supply-chain attestation; no human-verdict layer, no criteria-freeze semantics, no negative-control rule |
| ACM Artifact Badging / replication track | badges for available/reproduced artifacts | committee process; not a machine-consumable record with lock semantics |
| **A Trust Layer for Agent Evaluation** (arXiv:2610.07274, Oct 2026) | post-hoc audit of recorded agent scores; deterministic verdicts over evidence; "never overwrite the recorded score" — the closest academic analog | audits benchmark runs, not research claims; no criteria-lock, no negative-control rule, no human sign-off, no record format |
| [Treeship](https://github.com/zerkerlabs/treeship), [assay](https://github.com/metahub-ai/assay), NovaFabric | signed receipts/reports for agent runs, Merkle transparency | runtime provenance for agent workflows; not criteria-locked claim records |
| IETF drafts ATTP / AGTP (2026), OpenID Federation, Verifiable Trust | "trust layer" for identity and action governance of agents | identity/action governance; nothing about research claims or verification records |

The phrase "trust layer" is being colonised (2026) by security and identity
domains. The niche this protocol occupies — **criteria-locked,
machine-checked, human-signed verification records for research artifacts**
— is, as of the date above, unoccupied. This specification is an attempt to
fill it with a minimal file protocol, not a platform.

## Changes and incidents

[`CHANGELOG.md`](CHANGELOG.md) · [`trust-incidents.md`](trust-incidents.md)
(v3.5 freeze until 2027-02-04; changes only via incidents).
