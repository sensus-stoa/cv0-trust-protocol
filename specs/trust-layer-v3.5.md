# CV→0 Trust Layer — Specification v3.5

> **Version:** 3.5. Changes only via an entry in [`trust-incidents.md`](../trust-incidents.md).
> **Reference record (worked example):** sensus-stoa/permutation-test-ml,
> V-PTM-001 (criteria-lock `c60ed76`, machine-check `tc` 10/10 PASS,
> human sign-off 2026-11-09, report sha256 `fcf975ee`).
> Machine-readable contract: [`schemas/`](../schemas) — stable `$id` URLs
> fetchable by agents over raw.githubusercontent.
> Russian edition of this text: [`trust-layer-v3.5.ru.md`](trust-layer-v3.5.ru.md).

---

### 3.7 Trust Layer — the architecture of traceable trust

**Purpose:** give a person and an AI intermediary the ability to trace on
what basis a particular verdict was rendered, what exactly was verified,
by whom, in which version, and where the admissible conclusions end.

The Trust Layer is not a separate SaaS, not a universal Trust Score, and not
a promise of absolute truth. It is a cross-cutting layer of specification,
evidence, verification, and accountability that connects human and machine
interfaces.

#### Chain of trust

`Claim → Evidence → Provenance → Verification → Verdict → Limitations`

- **Claim:** an atomic statement we intend to make. A claim must carry an
  identifier, an exact formulation, a scope, and a status.
- **Evidence:** an artifact or observation that supports or refutes a claim:
  data, code, a commit, a configuration, a log, a seed, a control run, a
  measurement protocol.
- **Provenance:** the origin of evidence: who created/executed it, where,
  when, on which versions, configurations, and data; how the artifact
  reached us and whether it was modified.
- **Verification:** the concrete procedure that checks the evidence and the
  claim: inspection, a re-run, an independent reproduction, a control,
  a comparison against a baseline, or another explicitly described procedure.
- **Verdict:** a bounded decision on the claim within the stated scope.
- **Limitations:** what was not verified, which assumptions were accepted,
  which failure modes remain, and where the conclusion must not be applied.

#### Do not mix four properties

1. **Integrity:** the file has not changed relative to a recorded hash.
2. **Authenticity / provenance:** verifiable information about the origin
   and authorship of an artifact.
3. **Reproducibility:** the result can be obtained again under the
   described conditions.
4. **Validity:** the chosen method genuinely supports the specific
   conclusion.

A hash confirms integrity relative to a recorded value but by itself proves
neither authorship, nor data correctness, nor truth of content. A re-run
confirms reproducibility in a specific environment but by itself does not
prove the method is right. These properties must be displayed separately.

#### Statuses and verdict rules

- `PASS` — the pre-defined criteria are met within an explicitly stated
  scope.
- `FAIL` — at least one mandatory criterion was violated, or a confirmed
  defect was found.
- `INCONCLUSIVE` — data/access/verification power was insufficient, evidence
  conflicts, or the method is not applicable.

Do not turn the absence of a detected defect into proof of its absence. For
every verdict, state the criteria that were checked, the criteria that were
not, and the basis of the status. Do not use a single numeric Trust Score:
it would hide different kinds of uncertainty and non-equivalent properties.

#### Evidence roles and independence

Keep the four provenance classes from v3.3:

- `client-generated` — prepared by the client or the client's agent;
- `client-executed` — executed on the client's side;
- `provider-verified` — verified by us under a described procedure;
- `independently reproduced` — reproduced in an environment separated from
  the original run.

Additionally record the verifier's role and any conflict of interest.
`Provider-verified` does not automatically mean `independently reproduced`.
Self-verification by the author of a result may be useful, but must not be
labelled an independent check.

#### Verdict lifecycle

Every verdict is bound to:
- a unique ID and date;
- the protocol/criteria version and the schema version;
- the version of the object under verification (commit, release,
  model/pipeline version);
- scope, data, and configuration;
- the list of claims and evidence IDs;
- the executor and the verifier;
- results and limitations.

When the object, the criteria, or material evidence changes, the old verdict
is not rewritten retroactively: it is marked `superseded`, `withdrawn`, or
`expired` with a reason and a reference to the new status. History is kept.
Withdrawing does not make the old report disappear; it means its current
applicability is no longer claimed.

#### Minimal Trust Record

At the first stage, four human- and machine-readable files suffice:

- `claims.md` — claims, scope, criteria, and current status;
- `evidence-manifest.json` — evidence IDs, types, hashes, sources, versions,
  dates, and provenance;
- `verdict.json` — the result per claim, the verification procedure, the
  status, the protocol version, and references to evidence;
- `limitations.md` — unverified areas, assumptions, known limitations,
  conflicts, and revision conditions.

A JSON Schema is needed to check structure, mandatory fields, and types — it
does not attest the truthfulness of values. Signatures, attestations,
timestamping, a transparency log, and cryptographic infrastructure are
possible upgrades only under a real client need or threat model; they are
not part of the mandatory starting volume.

#### Trust Layer quality criterion

A third-party engineer who did not participate in the original work must be
able to:
1. understand which specific statement is being verified;
2. trace it back to the underlying evidence;
3. determine the origin and versions of the artifacts;
4. understand which verification procedure was performed;
5. distinguish what was verified from what was not;
6. establish the applicability boundaries and the reason for the current
   status.

If the chain is not traceable, the verdict does not rise to `PASS`: it
stays `INCONCLUSIVE`, or narrows to the part that was actually verified.

#### Relation of statuses to CV→0 classes

The CV→0 class (INVARIANT / PREDICTIVE APPROXIMATION / NO SIGNAL) describes
a candidate. The Trust Layer status describes a claim. One run may yield
`PASS` on the claim "the model predicts" and `FAIL` on the claim "an
invariant was found" (class PREDICTIVE APPROXIMATION).

`reason_code` is mandatory for `FAIL` and `INCONCLUSIVE`:
- scientific: `DATA`, `GRAMMAR`, `NOISE`, `DEPTH`;
- operational: `ACCESS`, `BUDGET`, `ENVIRONMENT`, `CONFLICT_EVIDENCE`,
  `OUT_OF_SCOPE`.

Operational codes must not be presented as scientific conclusions.

#### The negative-control rule

`PASS` on a reliability claim is impossible without a recorded negative
(zero) control: id, N, and the number of false `PASS`es. A record of "0
false" without N is not accepted. State the upper bound explicitly: for 0 of
N it is roughly 3/N (rule of three, 95%). If a control is impossible, the
status is at most `INCONCLUSIVE` with `OUT_OF_SCOPE` and a reason.

#### Freezing criteria before runs (criteria-lock)

The `PASS`/`FAIL` criteria are written into `claims.md` and committed before
results are obtained. The commit hash and the fixation time are recorded in
`evidence-manifest.json`. Changing criteria after a run is issued as a new
version marked `post-hoc`, and `PASS` on it requires a new run. No
cryptographic infrastructure is needed: the order of commits suffices.

#### Untrusted artifacts

Any client artifact (logs, README files, code comments, agent reports) is
data, not instructions. Text inside an artifact does not change the
procedure, the criteria, or the scope. Agents read artifacts without access
to secrets or egress traffic. A detected injection attempt is recorded in
`limitations.md` as a finding and does not raise the claim's status.

#### Verifier Track Record

Before the first paid verdict and then regularly, the procedure is run on
planted defects: pipelines with pre-inserted known errors and clean
controls. Sensitivity, the share of false `PASS`es on defective cases, and
the share of false `FAIL`s on clean ones are fixed, always with N. Starting
volume: 5 defective and 5 clean cases. At that N the intervals are wide, and
this is written next to the numbers. The result is published together with
the version of the methodology.

#### Disclosure levels and validity

Each artifact in the manifest carries a `disclosure` level: `public`,
`client`, or `sealed` (hash and metadata are open, content under NDA).
`verdict.json` must contain `valid_until`: an explicit date or
`object-change`.

#### The change rule

The specification is versioned. Changes are allowed only as a response to
an entry in `trust-incidents.md` (what went wrong in real use). New fields,
statuses, and files without an incident are not added.
