# LICENSES.md — file-to-license mapping (PROPOSED)

Status: **PROPOSED** — this mapping is part of the Zenodo candidate archive
and awaits the author's final confirmation before it is considered applied
to the `main` branch.

Rationale per the author's decision (2026-11-09): documentation and the
normative specification are CC BY 4.0 (attribution-based reuse); the checker
source, tests, and built artifacts are Apache-2.0 (includes an explicit
patent grant for covered contributions); JSON Schemas are treated as the
normative, machine-readable part of the specification and therefore follow
CC BY 4.0.

## Mapping

| Path pattern | License |
|---|---|
| `specs/**` | CC-BY-4.0 |
| `README.md`, `GUIDE.md`, `CHANGELOG.md`, `trust-incidents.md` | CC-BY-4.0 |
| `LICENSES.md` | CC-BY-4.0 |
| `schemas/*.json` | CC-BY-4.0 |
| `checker/**` (source, tests, testdata, install script) | Apache-2.0 |
| `releases/tc-*` (built artifacts) | Apache-2.0 |
| `releases/*.sha256` | CC-BY-4.0 |

License texts shipped in this archive:
- `LICENSES/LICENSE-Apache-2.0.txt`
- `LICENSES/LICENSE-CC-BY-4.0.txt`

## Third-party components

- Go standard library and toolchain: Go is distributed under a BSD-style
  license. The Go toolchain is not redistributed in this repository; nothing
  third-party is vendored into the checker source.
- No NOTICE file content from third parties is bundled; per
  Apache-2.0 §4(d), redistribution includes this LICENSES.md as the
  attribution notice for the checker files.
- No other third-party components are included in the archive.

## Trademark / project name

The name "CV→0 Trust Layer" (and the ASCII form "CV0 Trust Layer") is NOT
granted as a trademark by these licenses. Licensing covers the text and
code under the licenses above; use of the project name in derived materials
should avoid implying endorsement.

## Interpretation

- CC BY 4.0 covers human- and machine-readable narrative and normative
  documents.
- Apache-2.0 covers executable logic (the checker) executed from source or
  built artifacts.
- If a single license must be picked by an automated pipeline, treat the
  specification as CC BY 4.0 and the code as Apache-2.0.
