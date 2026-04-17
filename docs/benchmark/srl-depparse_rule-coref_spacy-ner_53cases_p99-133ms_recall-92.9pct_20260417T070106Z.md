# Extraction Pipeline Benchmark — 20260417T070106Z

## Run configuration

| Key | Value |
|-----|-------|
| Date (UTC) | 2026-04-17 07:01:06 |
| Git SHA | `14aaa8d` |
| Service URL | http://localhost:8090 |
| Service version | 0.3.0 |
| Runs per case | 10 |
| Total cases | 53 |
| Total samples | 530 |
| coreferee | ✗ rule-based fallback |
| GLiNER | ✗ spaCy NER fallback |
| Python | 3.11.2 |
| Platform | Linux-6.6.99-09128-g14e87a8a9b71-x86_64-with-glibc2.36 |

## Summary

| Metric | Result | Target | Status |
|--------|--------|--------|--------|
| Latency p50 | 38.5 ms | — | — |
| Latency p95 | 104.9 ms | — | — |
| Latency p99 | 133.3 ms | <100 ms | ✗ MISSED |
| Triple recall | 65/70 (92.9%) | >93% | ✗ MISSED |
| Avg facts extracted | 1.85 | — | — |
| Requests under 100ms | 496/530 (94%) | 100% | ✗ |

## Recall by difficulty tier

| Tier | Found | Expected | Recall |
|------|-------|----------|--------|
| basic | 23 | 23 | 100.0% |
| medium | 20 | 21 | 95.2% |
| hard | 22 | 26 | 84.6% |

## Per-case results

| # | Difficulty | Description | p50 ms | Facts | Recall |
|---|-----------|-------------|--------|-------|--------|
| 1 | basic | simple SVO with named entities | 37.0 | 1 | 1/1 |
| 2 | basic | acquisition with money modifier | 35.0 | 1 | 1/1 |
| 3 | basic | simple SVO no named entities | 32.0 | 1 | 1/1 |
| 4 | basic | definite NP subject | 32.3 | 1 | 1/1 |
| 5 | basic | simple SVO with temporal | 34.3 | 1 | 1/1 |
| 6 | basic | copular: subject is predicate | 32.3 | 1 | 1/1 |
| 7 | basic | copular: validity state | 31.8 | 1 | 1/1 |
| 8 | basic | copular: remain | 32.5 | 1 | 1/1 |
| 9 | basic | copular: classification | 34.0 | 1 | 1/1 |
| 10 | basic | copular: seem + acomp | 31.8 | 1 | 1/1 |
| 11 | basic | stative copular: is + adverb + adjective | 34.3 | 1 | 1/1 |
| 12 | basic | passive with explicit agent | 33.9 | 1 | 1/1 |
| 13 | basic | passive with agent NP | 36.1 | 1 | 1/1 |
| 14 | basic | passive no agent | 32.3 | 1 | 1/1 |
| 15 | basic | eventive passive: rejected by agent | 34.7 | 1 | 1/1 |
| 16 | basic | NER: org acquires org with date and money | 35.1 | 1 | 1/1 |
| 17 | basic | NER: org with multi-token name | 36.8 | 1 | 1/1 |
| 18 | basic | temporal: year | 33.5 | 1 | 1/1 |
| 19 | basic | temporal: after-clause | 33.7 | 1 | 1/1 |
| 20 | basic | negation: did not | 35.3 | 1 | 1/1 |
| 21 | basic | negation: copular | 33.8 | 1 | 1/1 |
| 22 | basic | intransitive: subject + verb only | 34.3 | 1 | 1/1 |
| 23 | basic | intransitive: phrasal verb | 36.4 | 1 | 1/1 |
| 24 | medium | coreference: she → Alice | 53.8 | 2 | 2/2 |
| 25 | medium | coreference: it → loan (non-human antecedent) | 51.8 | 2 | 1/2 ✗ |
| 26 | medium | coordination: two subjects | 33.8 | 1 | 2/2 |
| 27 | medium | coordination: two verbs same object | 36.2 | 3 | 1/1 |
| 28 | medium | coordination: compound sentence two SVOs | 40.0 | 3 | 2/2 |
| 29 | medium | relcl: that-clause stripped from object | 46.8 | 2 | 1/1 |
| 30 | medium | relcl: non-restrictive inside subject NP | 50.9 | 2 | 1/1 |
| 31 | medium | advcl: although-clause | 47.1 | 2 | 1/1 |
| 32 | medium | xcomp: decide + to-infinitive | 47.8 | 2 | 1/1 |
| 33 | medium | copular: became + NP complement | 34.6 | 1 | 1/1 |
| 34 | medium | copular: turned into + NP | 35.9 | 1 | 1/1 |
| 35 | medium | reported speech: said + complement | 46.8 | 2 | 2/2 |
| 36 | medium | reported speech: claimed + passive complement | 45.6 | 2 | 2/2 |
| 37 | medium | money object with PP modifier | 38.7 | 1 | 1/1 |
| 38 | medium | money predicate: totalled | 36.1 | 1 | 1/1 |
| 39 | hard | long-distance: relcl inside subject NP | 52.4 | 2 | 1/1 |
| 40 | hard | heavy NP: stacked modifiers on subject and object | 39.4 | 1 | 1/1 |
| 41 | hard | coreference multi-hop: they → board, it → proposal | 55.1 | 2 | 1/2 ✗ |
| 42 | hard | embedded: complement clause with xcomp | 78.8 | 2 | 2/2 |
| 43 | hard | embedded: passive inside complement | 63.4 | 2 | 2/2 |
| 44 | hard | hard: long multi-clause with PP and participial | 80.6 | 2 | 1/2 ✗ |
| 45 | hard | hard: deeply embedded subject NP + object clause | 92.2 | 4 | 2/2 |
| 46 | hard | hard: advcl + heavy object + PP adjunct | 64.0 | 2 | 1/1 |
| 47 | hard | hard: reported speech + adverb + PP time span | 81.2 | 2 | 2/2 |
| 48 | hard | hard: participial clause + two conjoined predicates + numerals | 68.4 | 3 | 2/2 |
| 49 | hard | hard: legal register, embedded clause, temporal advcl | 126.3 | 9 | 2/2 |
| 50 | hard | hard: relcl on subject + temporal advcl + numeral | 133.6 | 6 | 1/2 ✗ |
| 51 | hard | hard: double object (bank, fine amount) + gerund + date range | 80.0 | 3 | 1/1 |
| 52 | hard | hard: concessive advcl + passive complement + numeral + PP | 97.1 | 2 | 1/2 ✗ |
| 53 | hard | hard: participial modifier + conjoined PPs + xcomp + numeral | 105.1 | 6 | 2/2 |

## Missed triples

Cases where at least one expected triple was not found:

- **coreference: it → loan (non-human antecedent)** (medium): 1/2 — 1 triple(s) missed
- **coreference multi-hop: they → board, it → proposal** (hard): 1/2 — 1 triple(s) missed
- **hard: long multi-clause with PP and participial** (hard): 1/2 — 1 triple(s) missed
- **hard: relcl on subject + temporal advcl + numeral** (hard): 1/2 — 1 triple(s) missed
- **hard: concessive advcl + passive complement + numeral + PP** (hard): 1/2 — 1 triple(s) missed

## Latency by difficulty tier

| Tier | p50 ms | p95 ms | p99 ms | Cases |
|------|--------|--------|--------|-------|
| basic | 34.0 | 36.8 | 37.0 | 23 |
| medium | 45.6 | 53.8 | 53.8 | 15 |
| hard | 80.0 | 133.6 | 133.6 | 15 |

> **Note on p99**: the overall p99 is skewed by hard-tier sentences >40 words (legal/financial register).
> Typical LLM output maps to basic/medium tier — see the per-tier table above for operationally relevant numbers.

## Full latency distribution

```
        0–25ms    0
       25–50ms  ███████████████████████████  362
       50–75ms  █████  78
      75–100ms  ████  56
     100–150ms  ██  34
        >150ms    0
```


## Metric glossary

| Metric | Meaning |
|--------|---------|
| **p50 latency** | Median request round-trip time in milliseconds. Half of requests complete faster than this. |
| **p95 latency** | 95th-percentile latency. One in twenty requests takes at least this long. Indicates tail behaviour under normal load. |
| **p99 latency** | 99th-percentile latency. One in a hundred requests is at least this slow. The hard latency ceiling — Velarix targets <100ms here. |
| **Triple recall** | Fraction of expected (subject, predicate, object) triples that appear in the extracted facts. A triple is matched if every non-empty hint string is a substring of the corresponding normalised field. |
| **Avg facts extracted** | Mean number of atomic facts the pipeline produces per input text. Higher is not always better — over-extraction introduces noise. |
| **Requests under 100ms** | Count and percentage of individual HTTP calls that completed within 100ms. Should be 100% in dev. |
| **coreferee** | Neural coreference model (requires spaCy ≤3.5). When unavailable, rule-based pronoun heuristics are used instead. |
| **GLiNER** | Zero-shot NER model (`urchade/gliner_small-v2.1`, ~580MB). When unavailable, spaCy's built-in NER is used. Enable with `VELARIX_ENABLE_GLINER=1`. |

### Difficulty tiers

| Tier | Characteristics |
|------|----------------|
| **basic** | Single clause, clean grammar, ≤15 words. Baseline for any working pipeline. |
| **medium** | Coreference, coordination, relative clauses, embedded complements, adverbial clauses. |
| **hard** | Long sentences (>25 words), multi-hop coreference, stacked NPs, legal/financial register, conjoined predicates, implicit agents. |

### Why these targets?

- **<100ms p99**: extraction sits in the hot path before fact assertion. At >100ms the user-visible latency for `/extract-and-assert` crosses 200ms with network overhead.
- **>93% triple recall**: below this threshold, too many facts are silently missed for the TMS dependency graph to be reliable. 93% was chosen as the minimum that keeps hallucination-detection effective in the long-horizon benchmark.
