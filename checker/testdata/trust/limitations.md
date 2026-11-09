# tc self-test fixture: known limitations

This fixture record exists only to exercise the trust_check validator
(checker/ tests). Nothing here asserts anything about any real system.

- The negative-control numbers are fixture values, not measurements.
- C-03 is deliberately INCONCLUSIVE with reason_code OUT_OF_SCOPE.
- The criteria_lock commit ids are fixture strings, not real commits;
  the validator treats fixture roots without .git as SKIP for lock checks
  and the tests build a dedicated git fixture for the I-001 regression.
