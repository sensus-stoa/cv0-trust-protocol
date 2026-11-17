# Changelog — CV→0 Trust Layer protocol

Format: date, spec version, the change, the incident entry that triggered it
(if any). The spec is frozen for the cycle; changes only via
[`trust-incidents.md`](trust-incidents.md).

## tc v0.2.0 — 2026-11-17 (checker only; spec text v3.5 unchanged)

- Ten new checks, all backward compatible (records without the new optional
  fields still validate; the checks SKIP, they are never silently absent
  from the report): AUDIT_SHAPE, AUDIT_DECISION_XREF, CLAIM_PROMISE,
  REQUIRED_METRICS, EVIDENCE_SECTION, EXTERNAL_REF_WHITELIST, PATH_HYGIENE,
  VERSION_CONSISTENCY, LOCK_FIDELITY, OBJECT_REACHABLE.
- LOCK_FIDELITY: working-tree `trust/claims.md` must remain byte-identical
  to the blob at the criteria-lock commit — a post-lock criteria edit is a
  post-hoc change requiring a new record version and a full re-run.
- LOCK_BLOB/LOCK_ANCESTOR/LOCK_FIDELITY/OBJECT_REACHABLE distinguish a
  dangling pointer (history rewritten, old objects pruned) from an
  off-branch lock; OBJECT_REACHABLE verifies the pinned object.commit
  exists in the repository.
- CLAIM_PROMISE: pre-run audit promises are declared in the LOCKED
  claims.md (`Promise: case=… audits=… evidence=…`) and enforced against
  the run-output evidence; verdict-side promise fields alone are not a
  promise (an agent could fill them after the run).
- REQUIRED_METRICS and EXTERNAL_REF_WHITELIST are manifest-declared
  contracts (data, not code): tc stays domain-agnostic; project specifics
  (MAE, windows, cadences) belong to the object repository's own test
  contour.
- HASHES_MATCH now also verifies the hashes of manifest `disclosures`.
- The report carries `checked_commit` (HEAD of the checked work tree) and
  records `root` as passed on the command line — no absolute-path leak into
  the published report.
- Self-tests extended: every new check ships a positive and a planted-defect
  case (N=17 total), including the post-lock-edit and dangling-pointer
  regressions.
- Motivation: the independent review of an object record (V-EVTR-004)
  found defects invisible to tc v0.1.0 because they lived in run-output
  artifacts and in the audit battery itself; v0.2.0 closes the mechanical
  part of that finding list (see the object repository's
  CHECKABLE_FINDINGS.md for the classification).

## v3.5 — 2026-11-08 (initial release)

- First release of the Trust Layer specification (v3.5).
- Minimal record: `claims.md`, `evidence-manifest.json`, `verdict.json`,
  `limitations.md`.
- Rules: criteria-lock before runs; `reason_code` mandatory for
  FAIL/INCONCLUSIVE; `PASS` on reliability claims requires a negative
  control with N ≥ 1 (0/N → rule of three ≈ 3/N); Verifier Track Record on
  planted defects; disclosure levels (public/client/sealed); `valid_until`
  (date or object-change); append-only verdict history.
- Evidence provenance classes (from v3.3): client-generated,
  client-executed, provider-verified, independently reproduced.
- Machine-readable schemas: `schemas/evidence-manifest.schema.json`,
  `schemas/verdict.schema.json` (draft 2020-12) — structure and mandatory
  fields; the schema does not attest truthfulness of values. Both schemas
  validate the live worked-example record.
- Worked example: sensus-stoa/permutation-test-ml, record V-PTM-001
  (criteria-lock `c60ed76`; machine-check 10/10 PASS; human sign-off
  2026-11-09, report sha256 `fcf975ee`).
- Checker `tc` v0.1.0: Go 1.27.3, stdlib-only; vendored under `checker/`
  (source, fixture record in `checker/testdata/`, self-tests) and a
  prebuilt static binary under `releases/` (+ `.sha256`). Self-test N=5
  planted defects: all caught (see `docs/verifier-track-record.md` in the
  object repository). The checker's git-context check never walks up the
  directory tree (a tmp dir nested inside an unrelated checkout must not
  inherit its git context).
- Related-work survey recorded in README (CODECHECK, in-toto/SLSA, ACM
  badges, arXiv:2610.07274 "A Trust Layer for Agent Evaluation",
  Treeship/assay/NovaFabric, IETF ATTP/AGTP drafts): no accepted
  specification exists for criteria-locked, machine-checked, human-signed
  research records as of 2026-11-09.
- English edition of all documents (`specs/trust-layer-v3.5.ru.md` is a
  Russian edition of the spec text).
- Changes only via trust-incidents.md entries.
