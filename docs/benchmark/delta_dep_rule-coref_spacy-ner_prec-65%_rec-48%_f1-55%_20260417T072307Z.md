# Delta Dependency Benchmark — 20260417T072307Z

Measures how accurately stage 5 infers `depends_on` edges between facts.
Complements the triple-recall benchmark which measures stage 4 extraction.

## Run configuration

| Key | Value |
|-----|-------|
| Date (UTC) | 2026-04-17 07:23:07 |
| Git SHA | `14aaa8d` |
| Service URL | http://localhost:8090 |
| Service version | 0.4.0 |
| coreferee | ✗ rule-based fallback |
| GLiNER | ✗ spaCy NER fallback |
| Python | 3.11.2 |
| Platform | Linux-6.6.99-09128-g14e87a8a9b71-x86_64-with-glibc2.36 |

## Summary

| Metric | Result | Target |
|--------|--------|--------|
| Edge precision | 64.7% | >70% |
| Edge recall    | 47.8% | >70% |
| F1             | 55.0% | >70% |
| True positives | 11 | — |
| False positives | 6 | — |
| False negatives | 12 | — |
| Negative cases correct (no spurious edge) | 3/3 | — |

## Per-mechanism breakdown

| Mechanism | TP | FP | FN | Precision | Recall | F1 |
|-----------|----|----|----|-----------+--------|-----|
| connective | 0 | 0 | 0 | 0.0% | 0.0% | 0.0% |
| entity_overlap | 5 | 6 | 9 | 45.5% | 35.7% | 40.0% |
| chain | 5 | 0 | 0 | 100.0% | 100.0% | 100.0% |
| coreference | 1 | 0 | 3 | 100.0% | 25.0% | 40.0% |

## Per-case results

| # | Mechanism | Description | Expected edges | TP | FP | FN | Result |
|---|-----------|-------------|----------------|----|----|-----|--------|
| 1 | connective | causal: therefore links vendor-fail → contract-terminated | 1 | 0 | 1 | 1 | ✗ |
| 2 | connective | causal: because clause — invoice disputed → payment delayed | 1 | 0 | 1 | 1 | ✗ |
| 3 | connective | causal: consequently links merger-approved → shareholders-received | 1 | 0 | 0 | 1 | ~ FN |
| 4 | connective | contrastive: although — payment delayed, vendor fulfilled | 1 | 0 | 1 | 1 | ✗ |
| 5 | connective | temporal: after — report submitted → board approved | 1 | 0 | 1 | 1 | ✗ |
| 6 | connective | temporal: following — acquisition → expansion (optional, participial) | 0 | 0 | 0 | 0 | ✓ |
| 7 | connective | conditional: provided that — regulator approval gates deal | 1 | 0 | 1 | 1 | ✗ |
| 8 | connective | causal: because — audit-revealed → system-shut-down | 1 | 0 | 1 | 1 | ✗ |
| 9 | connective | temporal: subsequently — contract signed → project commenced | 1 | 0 | 0 | 1 | ~ FN |
| 10 | connective | contrastive: however — fine issued, bank appealed | 1 | 0 | 0 | 1 | ~ FN |
| 11 | entity_overlap | overlap: GitHub appears in both facts | 1 | 1 | 0 | 0 | ✓ |
| 12 | entity_overlap | overlap: same subject (board) in two facts | 1 | 1 | 0 | 0 | ✓ |
| 13 | entity_overlap | overlap: contract appears as object then subject | 1 | 1 | 0 | 0 | ✓ |
| 14 | entity_overlap | overlap: bank shared across both facts | 1 | 1 | 0 | 0 | ✓ |
| 15 | entity_overlap | overlap: goods as object then subject (also chain candidate) | 1 | 1 | 0 | 0 | ✓ |
| 16 | chain | chain: Activision is object of acquisition, subject of release | 1 | 1 | 0 | 0 | ✓ |
| 17 | chain | chain: proposal as object → subject | 1 | 1 | 0 | 0 | ✓ |
| 18 | chain | chain: fraud as object → subject (causal chain) | 1 | 1 | 0 | 0 | ✓ |
| 19 | chain | chain: injunction object → subject | 1 | 1 | 0 | 0 | ✓ |
| 20 | chain | chain: patent object → subject | 1 | 1 | 0 | 0 | ✓ |
| 21 | coreference | coref: she → Alice; alice-submit is parent of alice-included | 1 | 1 | 0 | 0 | ✓ |
| 22 | coreference | coref: they → board, it → proposal | 1 | 0 | 0 | 1 | ~ FN |
| 23 | coreference | coref: he → CEO; announce is parent of resigned | 1 | 0 | 0 | 1 | ~ FN |
| 24 | coreference | coref: she → manager | 1 | 0 | 0 | 1 | ~ FN |
| 25 | none | negative: completely unrelated facts should produce no edges | 0 | 0 | 0 | 0 | ✓ |
| 26 | none | negative: two independent state facts with no shared entity | 0 | 0 | 0 | 0 | ✓ |

## False positives (spurious edges)

- **causal: therefore links vendor-fail → contract-terminated**: 1 spurious edge(s) — 'The vendor fail' → 'The vendor deliver'
- **causal: because clause — invoice disputed → payment delayed**: 1 spurious edge(s) — 'The payment delay' → 'the invoice dispute'
- **contrastive: although — payment delayed, vendor fulfilled**: 1 spurious edge(s) — 'the vendor fulfil' → 'the payment delay'
- **temporal: after — report submitted → board approved**: 1 spurious edge(s) — 'The board approve' → 'the due diligence report submit'
- **conditional: provided that — regulator approval gates deal**: 1 spurious edge(s) — 'The deal proceed' → 'the regulators approve'
- **causal: because — audit-revealed → system-shut-down**: 1 spurious edge(s) — 'The system shut' → 'the audit reveal'

## False negatives (missed edges)

- **causal: therefore links vendor-fail → contract-terminated**: 1 edge(s) missed — edge missing: 'The vendor fail' → 'The contract terminate'
- **causal: because clause — invoice disputed → payment delayed**: 1 edge(s) missed — edge missing: 'the invoice dispute' → 'The payment delay'
- **causal: consequently links merger-approved → shareholders-received**: 1 edge(s) missed — edge missing: 'The merger approve' → 'the shareholders receive'
- **contrastive: although — payment delayed, vendor fulfilled**: 1 edge(s) missed — edge missing: 'the payment delay' → 'the vendor fulfil'
- **temporal: after — report submitted → board approved**: 1 edge(s) missed — edge missing: 'the due diligence report submit' → 'The board approve'
- **conditional: provided that — regulator approval gates deal**: 1 edge(s) missed — edge missing: 'the regulators approve' → 'The deal proceed'
- **causal: because — audit-revealed → system-shut-down**: 1 edge(s) missed — edge missing: 'the audit reveal' → 'The system shut'
- **temporal: subsequently — contract signed → project commenced**: 1 edge(s) missed — edge missing: 'The contract sign' → 'the project commence'
- **contrastive: however — fine issued, bank appealed**: 1 edge(s) missed — edge missing: 'The fine issue' → 'the bank appeal'
- **coref: they → board, it → proposal**: 1 edge(s) missed — fact not extracted: parent=EdgeHint(subject='board', predicate='review', object='proposal'), child=EdgeHint(subject='board', predicate='reject', object='proposal')
- **coref: he → CEO; announce is parent of resigned**: 1 edge(s) missed — fact not extracted: parent=EdgeHint(subject='ceo', predicate='announc', object='restructur'), child=EdgeHint(subject='ceo', predicate='resign', object='')
- **coref: she → manager**: 1 edge(s) missed — fact not extracted: parent=EdgeHint(subject='manager', predicate='approv', object='budget'), child=EdgeHint(subject='manager', predicate='notif', object='team')

## Metric glossary

| Metric | Meaning |
|--------|---------|
| **Edge precision** | Of all dependency edges Delta proposed, what fraction are in the ground truth? High precision means few spurious links. |
| **Edge recall** | Of all expected dependency edges, what fraction did Delta find? High recall means few missed links. |
| **F1** | Harmonic mean of precision and recall. The primary headline metric for dependency accuracy. |
| **TP** | True positive — an expected edge that Delta correctly produced. |
| **FP** | False positive — an edge Delta produced that is not in the ground truth. |
| **FN** | False negative — an expected edge that Delta missed entirely. |
| **Negative case** | A test case with no expected edges. Checks that the pipeline does not invent spurious links between unrelated facts. |

### Mechanisms

| Mechanism | Source | Confidence |
|-----------|--------|------------|
| connective | Discourse markers (because, although, after, therefore…) | 0.90 |
| entity_overlap | Shared entity string between two facts | 0.45–0.88 (entity-type weighted) |
| chain | Fact A's object == Fact B's subject (object→subject chain) | 0.80 |
| coreference | Resolved coreference chain (she → Alice) | inherited from coref model |