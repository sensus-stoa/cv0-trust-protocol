# Trust Layer protocol — incidents

This is the journal of SPEC changes. Under the freeze rule (v3.5, §3.7),
specification changes within a cycle are allowed only as a response to a
recorded incident here: what went wrong in real use, what lesson was
learned, and what it turned into. Record-level incidents (a broken record
in some object repository) live in that repository's
`trust/trust-incidents.md`; only incidents that expose a defect or a gap in
the SPEC itself land in this journal.

## P-001 — 2026-11-09 — empty criteria-lock blob

- **Source:** incident I-001 in sensus-stoa/permutation-test-ml (full entry:
  `trust/trust-incidents.md` in that repository).
- **Symptom:** the criteria-lock commit contained an EMPTY
  `trust/claims.md` (git blob `e69de29…`); the "locked" criteria text did
  not exist in any commit, and the GitHub landing page displayed an empty
  file behind a criteria-lock label.
- **Cause:** the criteria text was drafted and agreed in the working
  session but never written to the file before the lock commit; `git add`
  captured the empty file. No non-empty version existed anywhere in
  history.
- **Spec lesson:** a criteria-lock is valid only if the lock commit contains
  a NON-EMPTY `claims.md` AND is an ancestor of HEAD. No verification run
  counts against an invalid lock. The validator must enforce both
  mechanically (reference implementations: `tc` checks LOCK_BLOB and
  LOCK_ANCESTOR), and the checker must know how to FAIL on this defect
  (regression test I-001).
- **Status:** spec v3.5 text unchanged — the lesson is enforced in the
  validator and recorded here. The object's lock validity from the freeze
  date was unaffected: the criteria text existed byte-for-byte (restoring
  identical text is not a criteria change), the re-lock `c60ed76` carries
  identical text, and no re-runs were required.
- **Validator hardening beyond the spec text:** the checker never walks up
  the directory tree to discover a git context — a record criterion
  validated inside a tmp directory nested in an unrelated checkout must not
  inherit that checkout's git state (found while dogfooding the checker's
  own fixture tests).
