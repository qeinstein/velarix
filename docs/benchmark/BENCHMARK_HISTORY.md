# Delta — Benchmark History

All runs of the Delta extraction pipeline against the ground-truth dataset.
Each row links to the full per-case breakdown. Keep this file updated every time you
run the benchmark.

## Run index

| # | Date (UTC) | Service ver | Pipeline config | Cases | Runs/case | p50 ms | p95 ms | p99 ms | Recall | Notes | Full report |
|---|-----------|------------|----------------|-------|-----------|--------|--------|--------|--------|-------|-------------|
| 1 | 2026-04-17 | v0.2.0 | srl-depparse · rule-coref · spacy-ner | 44 | 5 | 33.7 | 55.9 | 72.7 | 92.6% (25/27) | Original 44-case suite, pre-conjoined-predicate fix | [report](extraction_benchmark_20260417T064138Z.md) |
| 2 | 2026-04-17 | v0.2.0 | srl-depparse · rule-coref · spacy-ner | 53 | 5 | 39.4 | 84.1 | 115.7 | 90.0% (63/70) | Expanded to 53 cases with hard tier; conjoined fix not yet applied | [report](srl-depparse_rule-coref_spacy-ner_53cases_p99-114ms_recall-91.4pct_20260417T064527Z.md) |
| 3 | 2026-04-17 | v0.2.0 | srl-depparse · rule-coref · spacy-ner | 53 | 5 | 38.6 | 89.9 | 113.7 | 91.4% (64/70) | Added conjoined verb expansion (inherited subject) | [report](srl-depparse_rule-coref_spacy-ner_53cases_p99-114ms_recall-91.4pct_20260417T064527Z.md) |
| 4 | 2026-04-17 | v0.2.0 | srl-depparse · rule-coref · spacy-ner | 53 | 5 | 38.6 | 111.8 | 150.4 | 92.9% (65/70) | Added shared-object inheritance for head verb; 5-run sample too small for stable p99 | [report](srl-depparse_rule-coref_spacy-ner_53cases_p99-150ms_recall-92.9pct_20260417T064616Z.md) |
| 5 | 2026-04-17 | v0.2.0 | srl-depparse · rule-coref · spacy-ner | 53 | 10 | 38.8 | 125.5 | 150.1 | 92.9% (65/70) | 10-run stable baseline; p99 skewed by 2 sentences >40 words | [report](srl-depparse_rule-coref_spacy-ner_53cases_p99-150ms_recall-92.9pct_20260417T064855Z.md) |
| 6 | 2026-04-17 | v0.3.0 | srl-depparse · rule-coref · spacy-ner | 53 | 10 | 38.6 | 104.9 | 133.1 | 92.9% (65/70) | v0.3.0: nlp.pipe() batching in stage4, senter pipe disabled, 60-token length cap | [report](srl-depparse_rule-coref_spacy-ner_53cases_p99-133ms_recall-92.9pct_20260417T070009Z.md) |
| 7 | 2026-04-17 | v0.4.0 | srl-depparse · rule-coref · spacy-ner | 53 | 10 | 42.3 | 98.3 | 125.1 | 92.9% (65/70) | v0.4.0: stage 5 dependency overhaul — index bug fix, word-boundary connectives, det-stripping, entity-type weighting, chain edges (5E), coref edges (5F), expanded connectives.json | [report](delta_rule-coref_spacy-ner_53cases_p99-125ms_recall-92.9pct_20260417T071847Z.md) |
| 8 | 2026-04-17 | v0.4.0 | srl-depparse · neural-coref · spacy-ner | 53 | 10 | 28.9 | 91.7 | 103.2 | 92.9% (65/70) | Downgraded spaCy 3.7.6 → 3.5.4; coreferee neural coref now active. p50 −13ms vs run 7 (coreferee reuses the same Doc pass). Recall unchanged — remaining misses are all non-personal/multi-hop coref and deep passive. | [report](delta_neural-coref_spacy-ner_53cases_p99-103ms_recall-92.9pct_20260417T073427Z.md) |
| 9 | 2026-04-17 | v0.5.0 | srl-depparse · neural-coref · spacy-ner | 53 | 10 | 30.2 | 84.5 | 99.2 | 92.9% (65/70) | v0.5.0: cross-sentence connective pass (sentence-opening connectives now fire), entity-length filter (single-token common nouns no longer anchor overlap edges), non-human "it" heuristic in rule-based coref. p99 now just under 100ms target. Dep F1 76%→84%. | [report](delta_neural-coref_spacy-ner_53cases_p99-99ms_recall-92.9pct_20260417T100056Z.md) |
| 10 | 2026-04-17 | v0.6.0 | srl-depparse · neural-coref · spacy-ner | 53 | 10 | 29.0 | 78.6 | 95.0 | 92.9% (65/70) | v0.6.0: contrastive direction fix (subordinate→main for all connective types), same-role entity overlap always qualifies, "they/them/their" non-human heuristic added to rule-based coref. Dep F1 84%→89%. p99 95ms. | [report](delta_neural-coref_spacy-ner_53cases_p99-95ms_recall-92.9pct_20260417T101542Z.md) |

## Latency by tier (run 5 — stable baseline)

| Tier | Sentence length | p50 ms | p95 ms | p99 ms | vs target |
|------|----------------|--------|--------|--------|-----------|
| basic | ≤15 words | 34 | 38 | 41 | ✓ |
| medium | 15–25 words | 51 | 58 | 63 | ✓ |
| hard ≤35w | 25–35 words | 84 | 127 | 127 | ~ |
| hard >40w | >40 words | 109 | 121 | 121 | ✗ |

Target: p99 < 100ms. Basic and medium tiers (representative of typical LLM output) are comfortably within target. The overall p99 is skewed by two extreme-length legal/financial sentences.

## Recall progression

| Run | Cases | Recall | Delta | What changed |
|-----|-------|--------|-------|--------------|
| 1 | 44 | 92.6% | — | Baseline (44-case suite) |
| 2 | 53 | 90.0% | −2.6pp | Expanded to 53 cases (harder test) |
| 3 | 53 | 91.4% | +1.4pp | Conjoined verb expansion (inherited subject) |
| 4–5 | 53 | 92.9% | +1.5pp | Shared-object inheritance for head verb |
| 6 | 53 | 92.9% | 0pp | v0.3.0 perf optimizations (recall unchanged) |

## Latency progression (p99)

| Run | p99 ms | Delta | What changed |
|-----|--------|-------|--------------|
| 1 (44 cases) | 72.7 | — | Baseline |
| 2 (53 cases) | 115.7 | +43ms | Expanded test suite with harder sentences |
| 5 | 150.1 | +34ms | Stable 10-run measurement |
| 6 | 133.1 | −17ms | v0.3.0: nlp.pipe() batching, senter disabled, 60-token cap |

## Remaining misses (run 5)

All 5 remaining misses are coreference failures — non-human and multi-hop pronoun resolution that rule-based coref cannot handle. Neural coreferee would fix these but requires spaCy ≤3.5 (incompatible with current stack).

| Case | Difficulty | Miss | Root cause |
|------|-----------|------|-----------|
| coreference: it → loan | medium | `it` not resolved to `loan` | Non-human antecedent, rule-based coref only handles personal pronouns |
| coreference multi-hop: they/it | hard | `it → proposal` missed | Multi-hop non-human coref |
| hard: long multi-clause | hard | `deal valued at` — `valued` no nsubj | Participial clause, implicit agent |
| hard: relcl on subject + advcl | hard | `price dropped` loses subject | Long sentence over-segmentation |
| hard: concessive advcl + passive | hard | `dollars diverted` — nsubjpass lost in nested complement | Deep passive in nested complement clause |

## GLiNER comparison (pending)

GLiNER (`urchade/gliner_small-v2.1`) improves zero-shot NER coverage over spaCy's built-in model,
which should improve entity-type annotation accuracy and entity boundary detection.

**Status**: Cannot benchmark on this dev machine — GLiNER requires ~580MB RAM headroom and the
machine has 2.6GB total / 1.1GB available (after spaCy + FastAPI + the running service). Loading
GLiNER triggers an OOM kill.

**To run the GLiNER comparison on a production machine:**
```bash
# Ensure ≥1.5GB free RAM, then:
VELARIX_ENABLE_GLINER=1 python main.py
python benchmark.py --runs 10
```
Add the resulting row to the Run index above with config `srl-depparse · rule-coref · gliner-ner`
and note the recall/latency delta vs run 6 (the spaCy NER baseline).

Expected impact: +1–3pp recall on entity-heavy cases (NER tier), minimal latency change
(GLiNER runs at startup, inference is a forward pass on pre-loaded weights — adds ~5–15ms per request).

## How to add a new run

1. Start the SRL service: `cd extractor/srl_service && python main.py`
2. Run the benchmark: `python benchmark.py --runs 10`
3. Copy the summary row from the printed output into the **Run index** table above.
4. Update **Recall progression** and **Remaining misses** if anything changed.
