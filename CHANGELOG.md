# Changelog — CV→0 Trust Layer protocol

Формат: дата, версия спеки, суть изменения, запись инцидента (если послужило
причиной). Спека заморожена на цикл; изменения — только через
[`trust-incidents.md`](trust-incidents.md).

## v3.5 — 2026-11-08 (initial freeze)

- Первый релиз спецификации Trust Layer (источник: Roadmap CV→0 v3.5, §3.7).
- Минимальный рекорд: `claims.md`, `evidence-manifest.json`, `verdict.json`,
  `limitations.md`.
- Правила: criteria-lock до запуска; reason_code обязателен для
  FAIL/INCONCLUSIVE; PASS по надёжности требует отрицательного контроля с
  N ≥ 1 (0/N → правило трёх ≈ 3/N); Verifier Track Record на посаженных
  дефектах; уровни disclosure (public/client/sealed); `valid_until`
  (дата либо object-change); append-only история вердиктов.
- Классы происхождения evidence (из v3.3): client-generated,
  client-executed, provider-verified, independently reproduced.
- Машиночитаемые схемы: `schemas/evidence-manifest.schema.json`,
  `schemas/verdict.schema.json` (draft 2020-12) — структура и обязательные
  поля; правдивость значений схема не удостоверяет.
- Работающий пример: sensus-stoa/permutation-test-ml, рекорд V-PTM-001
  (criteria-lock `c60ed76`; machine-check 10/10 PASS; human sign-off
  2026-11-09, report sha256 `fcf975ee`); валидатор `tc` — Go 1.27.3,
  stdlib-only, self-test N=5 негатив-контролей.
- Freeze до 2027-02-04; изменения — только через trust-incidents.md.
