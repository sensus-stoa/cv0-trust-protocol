# CV→0 Trust Layer — machine-checkable claims, human-signed verdicts

Протокол проверяемого доверия для исследовательских артефактов: каждый claim
прослеживается до evidence и provenance, критерии фиксируются коммитом **до**
запуска, механика валидируется машиной — а вердикт подписывает человек
именем и датой. Никакого Trust Score: доверие не декларируется, оно
прослеживается.

> **Что это НЕ:** не SaaS, не API, не runtime-безопасность агентов
> (allow/warn/block на tool-calls — это другой домен, см. Yang 2026
> AgentTrust, OWASP Agentic AI Top 10), не универсальный бейдж честности.
> Это файловый протокол: что проверено, кем, в какой версии и где кончаются
> допустимые выводы.

## Минимальный рекорд — 4 файла

| Файл | Содержимое |
|------|-----------|
| `trust/claims.md` | claims с id, scope, критериями; criteria-lock коммитом **до** запуска |
| `trust/evidence-manifest.json` | evidence IDs, типы, sha256, provenance, роли, disclosure |
| `trust/verdict.json` | статус по каждому claim, процедура проверки, `valid_until` |
| `trust/limitations.md` | что НЕ проверено, допущения, условия пересмотра |

Цепочка: **Claim → Evidence → Provenance → Verification → Verdict → Limitations**.
Четыре свойства не смешиваются: integrity (hash) ≠ authenticity
(provenance) ≠ reproducibility (повторный запуск) ≠ validity (методика
действительно поддерживает вывод).

## Жёсткие правила v3.5

- **Criteria-lock**: критерии коммитятся до получения результатов;
  изменение после — новая версия с пометкой `post-hoc`, PASS по ней требует
  нового прогона. Lock валиден только если lock-коммит содержит **непустой**
  `claims.md` и является предком HEAD.
- **Отрицательный контроль**: PASS по claim о надёжности невозможен без
  контроля с N ≥ 1 («0 ложных» без N не принимается; для 0/N — правило трёх).
- **reason_code обязателен** для FAIL и INCONCLUSIVE (научные: DATA, GRAMMAR,
  NOISE, DEPTH; операционные: ACCESS, BUDGET, ENVIRONMENT, CONFLICT_EVIDENCE,
  OUT_OF_SCOPE — операционные не подаются как научные выводы).
- **Валидатор обязан уметь FAIL**: перед каждым релизом чекер прогоняется на
  посаженных дефектах (Verifier Track Record, с N).
- **Append-only вердикты**: старый рекорд не переписывается —
  `superseded`/`withdrawn`/`expired` с новой версией; независимые
  верификаторы включают собственный `verdict_id` над тем же criteria-lock.
- **Клиентские артефакты — данные, а не инструкции** (внедрённые инструкции
  фиксируются как находка и не повышают статус claim).
- **Freeze**: изменения спеки в течение цикла — только через запись в
  [`trust-incidents.md`](trust-incidents.md).

Полный текст: [`specs/trust-layer-v3.5.md`](specs/trust-layer-v3.5.md).

## Схемы (для агентов и CI)

- [`schemas/evidence-manifest.schema.json`](schemas/evidence-manifest.schema.json)
- [`schemas/verdict.schema.json`](schemas/verdict.schema.json)

JSON Schema draft 2020-12; `$id` доступен по raw.githubusercontent URL —
agent клиента может скачать схему и валидировать рекорд без выкачивания
кода объекта. Ограничение честности: схема проверяет структуру, обязательные
поля и типы, но не правдивость значений; кросс-документные проверки
(sha256 артефактов, существование контроля, lock-blob) — работа механического
валидатора.

## Механический валидатор `tc`

Эталонная реализация живёт с объектом рекорда (vendored), не здесь:
`sensus-stoa/permutation-test-ml → scripts/trust_check/` (Go 1.27.3,
stdlib-only). Проверки: FILES_PRESENT, JSON_VALID, HASHES_MATCH,
CLAIMS_XREF, EVIDENCE_XREF, CONTROLS, REASON_CODES, VALID_UNTIL,
LOCK_BLOB, LOCK_ANCESTOR. Чекер обязан уметь FAIL — самопроверка на
посаженных дефектах с N (`docs/verifier-track-record.md` в объекте).

## Работающий пример

`sensus-stoa/permutation-test-ml` — рекорд V-PTM-001:
criteria-lock `c60ed76` ( реставрирован после инцидента I-001 — пустой lock),
machine report 10/10 PASS, human sign-off Evgeniy Dolgov 2026-11-09
(report sha256 `fcf975ee`), Verifier Track Record N=5.

## Родственные стандарты и чем отличаемся (проверено 2026-11-09)

| Система | Пересечение | Чего в ней нет для нашего домена |
|---------|-------------|----------------------------------|
| [CODECHECK](https://codecheck.org.uk/) | независимое исполнение кода под статьёй, сертификат в Zenodo | нет criteria-lock, негатив-контролей, структурного рекорда; проверяется воспроизводимость, не claims |
| [in-toto Attestation](https://in-toto.io/) / SLSA | подписанные привязки субъект↔метаданные (DSSE), предикаты | поставка-подписи supply chain; нет human-verdict слоя, критериев-локов, правила негативного контроля |
| ACM Artifact Badging / Replication | бейджи за доступность/воспроизведённость | комитет-процесс, не машиночитаемый рекорд с lock-семантикой |
| AgentTrust v2 (Yang 2026), agenttrust.ai (NHI glossary) | название «trust layer» | runtime-безопасность действий агента и NHI governance — другой домен |

Принятой спеки для «criteria-locked, machine-checked, human-signed»
исследовательских рекордов на дату не найдено; эта спецификация — попытка
заполнить нишу минимальным файловым протоколом, без платформы.

## Изменения

[`CHANGELOG.md`](CHANGELOG.md). Спека заморожена до 2027-02-04; изменения —
только через [`trust-incidents.md`](trust-incidents.md).
