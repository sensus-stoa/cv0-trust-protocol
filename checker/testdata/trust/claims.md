# Claims — tc self-test fixture record

(i) This record exists ONLY to test the trust_check validator. It lives in
checker/testdata/ of the protocol repository and asserts nothing outside it.

## C-01 Deterministic artifact reproduction

Claim: running `cat artifact.txt` prints the canonical fixture string.
Scope: this testdata directory only.
PASS criterion: output matches `trust-layer fixture artifact v1` byte-for-byte.
Verification V-01: the executor runs the single command and compares output.

## C-02 Statistical calibration (fixture)

Claim: the fixture negative control rejects H0 at a rate consistent with
alpha = 0.05.
PASS criterion: the Wilson 95% CI of the control rate contains alpha.
Verification V-02: the manifest lists negative control NC-01 with N and rate;
the verdict links claim -> control.

## C-03 Out-of-scope placeholder

Status INCONCLUSIVE, reason_code OUT_OF_SCOPE: no claim is made. Included so
the validator exercises the reason_code rule on a non-PASS claim.
