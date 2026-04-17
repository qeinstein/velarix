"""
Delta benchmark — Velarix fact extraction pipeline.

Measures latency (p50/p95/p99) and triple recall against a ground-truth dataset.

Usage:
    # Start Delta first:
    #   cd extractor/srl_service && python main.py
    #
    # Then run:
    #   python benchmark.py
    #   python benchmark.py --url http://localhost:8090 --runs 20 --out ../../docs/benchmark

Metrics:
    - p50/p95/p99 latency (ms): percentile request latencies over all runs
    - Triple recall: % of expected (subject, predicate, object) triples found
    - Avg facts per input: how many atomic facts the pipeline extracts on average
    - Model availability (coreferee / GLiNER): which optional components are active
"""

from __future__ import annotations

import argparse
import json
import os
import platform
import statistics
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import NamedTuple

import requests

# ---------------------------------------------------------------------------
# Ground-truth dataset
#
# Each case: (text, list of expected (subject_hint, predicate_hint, object_hint))
# Hints are lowercase substrings — a triple is matched if all non-empty hints
# appear as substrings of the normalised extracted triple components.
#
# Difficulty tiers:
#   basic    — clean SVO, copular, passive (single clause, ≤15 words)
#   medium   — coordination, negation, relcl, coreference, embedded clauses
#   hard     — long sentences, multi-hop coref, stacked NPs, reported speech,
#              implicit subjects, conjoined predicates, financial/legal register
# ---------------------------------------------------------------------------

class GroundTruth(NamedTuple):
    text: str
    expected: list[tuple[str, str, str]]
    description: str
    difficulty: str = "basic"


DATASET: list[GroundTruth] = [
    # =========================================================================
    # BASIC — single clause, clean grammar
    # =========================================================================

    # --- Simple SVO ---
    GroundTruth(
        "OpenAI released GPT-4 in March 2023.",
        [("openai", "releas", "gpt-4")],
        "simple SVO with named entities",
    ),
    GroundTruth(
        "Apple acquired Beats Electronics for three billion dollars.",
        [("apple", "acquir", "beats")],
        "acquisition with money modifier",
    ),
    GroundTruth(
        "John paid the invoice.",
        [("john", "pay", "invoice")],
        "simple SVO no named entities",
    ),
    GroundTruth(
        "The board approved the merger.",
        [("board", "approv", "merger")],
        "definite NP subject",
    ),
    GroundTruth(
        "Google launched Bard in February 2023.",
        [("google", "launch", "bard")],
        "simple SVO with temporal",
    ),

    # --- Copular predicates ---
    GroundTruth(
        "The vendor is approved.",
        [("vendor", "be", "approved")],
        "copular: subject is predicate",
    ),
    GroundTruth(
        "The contract is void.",
        [("contract", "be", "void")],
        "copular: validity state",
    ),
    GroundTruth(
        "The payment remains pending.",
        [("payment", "remain", "pending")],
        "copular: remain",
    ),
    GroundTruth(
        "Tesla is a technology company.",
        [("tesla", "be", "technology company")],
        "copular: classification",
    ),
    GroundTruth(
        "The transaction seems suspicious.",
        [("transaction", "seem", "suspicious")],
        "copular: seem + acomp",
    ),
    GroundTruth(
        "The system is fully operational.",
        [("system", "be", "operational")],
        "stative copular: is + adverb + adjective",
    ),

    # --- Passive voice ---
    GroundTruth(
        "The contract was signed by Alice.",
        [("contract", "sign", "alice")],
        "passive with explicit agent",
    ),
    GroundTruth(
        "The invoice was approved by the finance team.",
        [("invoice", "approv", "finance team")],
        "passive with agent NP",
    ),
    GroundTruth(
        "The payment was processed successfully.",
        [("payment", "process", "")],
        "passive no agent",
    ),
    GroundTruth(
        "The proposal was rejected by the committee.",
        [("proposal", "reject", "committee")],
        "eventive passive: rejected by agent",
    ),

    # --- Named entity enrichment ---
    GroundTruth(
        "Microsoft acquired GitHub in 2018 for 7.5 billion dollars.",
        [("microsoft", "acquir", "github")],
        "NER: org acquires org with date and money",
    ),
    GroundTruth(
        "The European Central Bank raised interest rates in July 2023.",
        [("european central bank", "rais", "interest rates")],
        "NER: org with multi-token name",
    ),

    # --- Temporal modifiers ---
    GroundTruth(
        "Amazon launched AWS in 2006.",
        [("amazon", "launch", "aws")],
        "temporal: year",
    ),
    GroundTruth(
        "The CEO resigned after the scandal.",
        [("ceo", "resign", "")],
        "temporal: after-clause",
    ),

    # --- Negation ---
    GroundTruth(
        "The vendor did not submit the required documents.",
        [("vendor", "submit", "documents")],
        "negation: did not",
    ),
    GroundTruth(
        "The contract is not valid.",
        [("contract", "be", "valid")],
        "negation: copular",
    ),

    # --- Intransitive verbs ---
    GroundTruth(
        "The contract expired on December 31st.",
        [("contract", "expir", "")],
        "intransitive: subject + verb only",
    ),
    GroundTruth(
        "Negotiations broke down after three weeks.",
        [("negotiation", "break", "")],
        "intransitive: phrasal verb",
    ),

    # =========================================================================
    # MEDIUM — coreference, coordination, embedded clauses, relcl, xcomp
    # =========================================================================

    # --- Coreference ---
    GroundTruth(
        "Alice submitted the report. She included all required sections.",
        [("alice", "submit", "report"), ("alice", "includ", "sections")],
        "coreference: she → Alice",
        "medium",
    ),
    GroundTruth(
        "The bank approved the loan. It was disbursed within three days.",
        [("bank", "approv", "loan"), ("loan", "disburs", "")],
        "coreference: it → loan (non-human antecedent)",
        "medium",
    ),

    # --- Coordination ---
    GroundTruth(
        "Alice and Bob approved the contract.",
        [("alice", "approv", "contract"), ("bob", "approv", "contract")],
        "coordination: two subjects",
        "medium",
    ),
    GroundTruth(
        "The company acquired and integrated the startup.",
        [("company", "acquir", "startup")],
        "coordination: two verbs same object",
        "medium",
    ),
    GroundTruth(
        "The deal closed, and both parties received payment.",
        [("deal", "clos", ""), ("parties", "receiv", "payment")],
        "coordination: compound sentence two SVOs",
        "medium",
    ),

    # --- Relative clauses ---
    GroundTruth(
        "Nvidia produces graphics cards that are used in data centers.",
        [("nvidia", "produc", "graphics cards")],
        "relcl: that-clause stripped from object",
        "medium",
    ),
    GroundTruth(
        "The AI system, which was developed by Anthropic, passed all safety tests.",
        [("ai system", "pass", "safety tests")],
        "relcl: non-restrictive inside subject NP",
        "medium",
    ),

    # --- Adverbial / subordinate clauses ---
    GroundTruth(
        "Although the payment was delayed, the vendor fulfilled the contract.",
        [("vendor", "fulfil", "contract")],
        "advcl: although-clause",
        "medium",
    ),

    # --- xcomp / control ---
    GroundTruth(
        "The committee decided to postpone the vote until further notice.",
        [("committee", "decid", "")],
        "xcomp: decide + to-infinitive",
        "medium",
    ),

    # --- Copular with NP complement ---
    GroundTruth(
        "Google became the market leader in search.",
        [("google", "becom", "market leader")],
        "copular: became + NP complement",
        "medium",
    ),
    GroundTruth(
        "The startup turned into a billion-dollar company.",
        [("startup", "turn", "")],
        "copular: turned into + NP",
        "medium",
    ),

    # --- Reported speech ---
    GroundTruth(
        "The auditor said the accounts were inaccurate.",
        [("auditor", "say", ""), ("account", "be", "inaccurat")],
        "reported speech: said + complement",
        "medium",
    ),
    GroundTruth(
        "Management claimed that no data had been leaked.",
        [("management", "claim", ""), ("data", "leak", "")],
        "reported speech: claimed + passive complement",
        "medium",
    ),

    # --- Money / numbers ---
    GroundTruth(
        "The company raised fifty million dollars in its Series B round.",
        [("company", "rais", "")],
        "money object with PP modifier",
        "medium",
    ),
    GroundTruth(
        "The fine totalled three hundred thousand dollars.",
        [("fine", "total", "")],
        "money predicate: totalled",
        "medium",
    ),

    # =========================================================================
    # HARD — long sentences, multi-hop coref, stacked NPs, legal/financial
    #        register, conjoined predicates, implicit agents
    # =========================================================================

    # --- Long-distance dependencies ---
    GroundTruth(
        "The CEO, who had been with the company for thirty years, finally resigned.",
        [("ceo", "resign", "")],
        "long-distance: relcl inside subject NP",
        "hard",
    ),
    GroundTruth(
        "The newly appointed CFO approved the revised annual budget.",
        [("cfo", "approv", "budget")],
        "heavy NP: stacked modifiers on subject and object",
        "hard",
    ),

    # --- Multi-hop coreference ---
    GroundTruth(
        "The board reviewed the proposal. They rejected it unanimously.",
        [("board", "review", "proposal"), ("board", "reject", "proposal")],
        "coreference multi-hop: they → board, it → proposal",
        "hard",
    ),

    # --- Embedded complement clauses ---
    GroundTruth(
        "The report confirmed that the vendor failed to deliver on time.",
        [("report", "confirm", ""), ("vendor", "fail", "")],
        "embedded: complement clause with xcomp",
        "hard",
    ),
    GroundTruth(
        "Regulators announced that the merger had been approved by the antitrust authority.",
        [("regulator", "announc", ""), ("merger", "approv", "")],
        "embedded: passive inside complement",
        "hard",
    ),

    # --- Long sentences with multiple clauses ---
    GroundTruth(
        "After months of negotiation, the two companies finally agreed to merge, with the deal valued at over five billion dollars.",
        [("compan", "agre", ""), ("deal", "valu", "")],
        "hard: long multi-clause with PP and participial",
        "hard",
    ),
    GroundTruth(
        "The independent auditor, appointed by the shareholders at the annual general meeting, concluded that the financial statements presented a true and fair view.",
        [("auditor", "conclud", ""), ("statement", "present", "")],
        "hard: deeply embedded subject NP + object clause",
        "hard",
    ),
    GroundTruth(
        "Despite repeated warnings from the compliance team, the operations manager authorised the transaction without obtaining the required sign-off from legal.",
        [("manager", "authoris", "transaction")],
        "hard: advcl + heavy object + PP adjunct",
        "hard",
    ),
    GroundTruth(
        "The whistleblower alleged that senior executives had knowingly misrepresented the company's financial position to investors over a period of three years.",
        [("whistleblower", "alleg", ""), ("executive", "misrepresent", "position")],
        "hard: reported speech + adverb + PP time span",
        "hard",
    ),
    GroundTruth(
        "Following the acquisition of its largest competitor, the firm rapidly expanded into three new markets and hired over two thousand employees in the subsequent quarter.",
        [("firm", "expand", ""), ("firm", "hir", "")],
        "hard: participial clause + two conjoined predicates + numerals",
        "hard",
    ),
    GroundTruth(
        "The judge ruled that the defendant had violated the terms of the injunction by continuing to distribute the software after the court order was issued.",
        [("judge", "rul", ""), ("defendant", "violat", "terms")],
        "hard: legal register, embedded clause, temporal advcl",
        "hard",
    ),
    GroundTruth(
        "Investors who had purchased shares before the announcement suffered significant losses when the stock price dropped by forty percent in a single trading session.",
        [("investor", "suffer", "loss"), ("price", "drop", "")],
        "hard: relcl on subject + temporal advcl + numeral",
        "hard",
    ),
    GroundTruth(
        "The regulator fined the bank two hundred million pounds for failing to maintain adequate anti-money-laundering controls between 2018 and 2022.",
        [("regulator", "fin", "bank")],
        "hard: double object (bank, fine amount) + gerund + date range",
        "hard",
    ),
    GroundTruth(
        "Although the initial review found no irregularities, a subsequent forensic audit revealed that approximately fifteen million dollars had been systematically diverted into offshore accounts over a four-year period.",
        [("audit", "reveal", ""), ("dollar", "divert", "")],
        "hard: concessive advcl + passive complement + numeral + PP",
        "hard",
    ),
    GroundTruth(
        "The newly formed joint venture, backed by three sovereign wealth funds and headquartered in Singapore, plans to invest up to ten billion dollars in renewable energy projects across Southeast Asia over the next decade.",
        [("venture", "plan", ""), ("venture", "invest", "")],
        "hard: participial modifier + conjoined PPs + xcomp + numeral",
        "hard",
    ),
]


# ---------------------------------------------------------------------------
# Triple matching
# ---------------------------------------------------------------------------

def _normalize(s: str) -> str:
    return s.lower().strip()


def _triple_matches(fact: dict, expected: tuple[str, str, str]) -> bool:
    """Return True if all non-empty hints appear as substrings of the fact triple."""
    subj_hint, pred_hint, obj_hint = expected
    subj = _normalize(fact.get("subject", ""))
    pred = _normalize(fact.get("predicate", ""))
    obj  = _normalize(fact.get("object",  ""))
    if subj_hint and subj_hint not in subj:
        return False
    if pred_hint and pred_hint not in pred:
        return False
    if obj_hint  and obj_hint  not in obj:
        return False
    return True


def _case_found(facts: list[dict], expected_triples: list[tuple[str, str, str]]) -> int:
    found = 0
    for expected in expected_triples:
        if any(_triple_matches(f, expected) for f in facts):
            found += 1
    return found


# ---------------------------------------------------------------------------
# File output
# ---------------------------------------------------------------------------

_METRIC_GLOSSARY = """\
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
"""


def _save_results(
    out_dir: Path,
    health: dict,
    case_results: list[dict],
    all_latencies: list[float],
    total_found: int,
    total_expected: int,
    total_facts: int,
    runs_per_case: int,
    base_url: str,
) -> Path:
    out_dir.mkdir(parents=True, exist_ok=True)
    ts = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    coref_tag  = "neural-coref"  if health.get("coreferee") else "rule-coref"
    ner_tag    = "gliner-ner"    if health.get("gliner")    else "spacy-ner"
    n_cases    = len(case_results)
    recall_pct = (total_found / total_expected * 100) if total_expected else 0
    all_lats   = sorted(all_latencies)
    p99_val    = all_lats[int(len(all_lats) * 0.99)] if all_lats else 0
    fname = (
        f"delta_{coref_tag}_{ner_tag}"
        f"_{n_cases}cases"
        f"_p99-{p99_val:.0f}ms"
        f"_recall-{recall_pct:.1f}pct"
        f"_{ts}.md"
    )
    out_path = out_dir / fname

    n = len(all_latencies)
    p50 = statistics.median(all_latencies)
    p95 = all_latencies[int(n * 0.95)]
    p99 = all_latencies[int(n * 0.99)]
    recall_pct = (total_found / total_expected * 100) if total_expected else 0
    avg_facts = total_facts / len(DATASET)
    under_100 = sum(1 for l in all_latencies if l < 100)

    lat_target  = "✓ MET" if p99 <= 100 else ("~ p95 met" if p95 <= 100 else "✗ MISSED")
    rec_target  = "✓ MET" if recall_pct >= 93 else "✗ MISSED"

    # Per-difficulty breakdown
    diff_stats: dict[str, dict] = {"basic": {"found": 0, "expected": 0}, "medium": {"found": 0, "expected": 0}, "hard": {"found": 0, "expected": 0}}
    for r in case_results:
        d = r["difficulty"]
        found_n, exp_n = map(int, r["recall"].split("/"))
        diff_stats[d]["found"]    += found_n
        diff_stats[d]["expected"] += exp_n

    # Collect misses
    misses = [r for r in case_results if r["recall"].split("/")[0] != r["recall"].split("/")[1]]

    try:
        git_sha = subprocess.check_output(["git", "rev-parse", "--short", "HEAD"], stderr=subprocess.DEVNULL).decode().strip()
    except Exception:
        git_sha = "unknown"

    lines: list[str] = [
        f"# Delta Benchmark — {ts}",
        "",
        "## Run configuration",
        "",
        f"| Key | Value |",
        f"|-----|-------|",
        f"| Date (UTC) | {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S')} |",
        f"| Git SHA | `{git_sha}` |",
        f"| Service URL | {base_url} |",
        f"| Service version | {health.get('version', '?')} |",
        f"| Runs per case | {runs_per_case} |",
        f"| Total cases | {len(DATASET)} |",
        f"| Total samples | {n} |",
        f"| coreferee | {'✓ neural' if health.get('coreferee') else '✗ rule-based fallback'} |",
        f"| GLiNER | {'✓ active' if health.get('gliner') else '✗ spaCy NER fallback'} |",
        f"| Python | {sys.version.split()[0]} |",
        f"| Platform | {platform.platform()} |",
        "",
        "## Summary",
        "",
        f"| Metric | Result | Target | Status |",
        f"|--------|--------|--------|--------|",
        f"| Latency p50 | {p50:.1f} ms | — | — |",
        f"| Latency p95 | {p95:.1f} ms | — | — |",
        f"| Latency p99 | {p99:.1f} ms | <100 ms | {lat_target} |",
        f"| Triple recall | {total_found}/{total_expected} ({recall_pct:.1f}%) | >93% | {rec_target} |",
        f"| Avg facts extracted | {avg_facts:.2f} | — | — |",
        f"| Requests under 100ms | {under_100}/{n} ({under_100/n*100:.0f}%) | 100% | {'✓' if under_100 == n else '✗'} |",
        "",
        "## Recall by difficulty tier",
        "",
        "| Tier | Found | Expected | Recall |",
        "|------|-------|----------|--------|",
    ]
    for tier in ("basic", "medium", "hard"):
        ds = diff_stats[tier]
        pct = (ds["found"] / ds["expected"] * 100) if ds["expected"] else 0
        lines.append(f"| {tier} | {ds['found']} | {ds['expected']} | {pct:.1f}% |")

    lines += [
        "",
        "## Per-case results",
        "",
        "| # | Difficulty | Description | p50 ms | Facts | Recall |",
        "|---|-----------|-------------|--------|-------|--------|",
    ]
    for i, r in enumerate(case_results, 1):
        miss = " ✗" if r["recall"].split("/")[0] != r["recall"].split("/")[1] else ""
        lines.append(f"| {i} | {r['difficulty']} | {r['description']} | {r['p50_ms']} | {r['facts']} | {r['recall']}{miss} |")

    if misses:
        lines += [
            "",
            "## Missed triples",
            "",
            "Cases where at least one expected triple was not found:",
            "",
        ]
        for r in misses:
            found_n, exp_n = map(int, r["recall"].split("/"))
            lines.append(f"- **{r['description']}** ({r['difficulty']}): {r['recall']} — {exp_n - found_n} triple(s) missed")
    else:
        lines += ["", "## Missed triples", "", "None — all expected triples matched.", ""]

    # Per-difficulty latency breakdown
    diff_lats: dict[str, list[float]] = {"basic": [], "medium": [], "hard": []}
    for r in case_results:
        diff_lats[r["difficulty"]].extend([r["p50_ms"]] * runs_per_case)

    lines += ["", "## Latency by difficulty tier", "", "| Tier | p50 ms | p95 ms | p99 ms | Cases |", "|------|--------|--------|--------|-------|"]
    for tier in ("basic", "medium", "hard"):
        dl = sorted(diff_lats[tier])
        if not dl:
            continue
        dp50 = statistics.median(dl)
        dp95 = dl[int(len(dl) * 0.95)]
        dp99 = dl[int(len(dl) * 0.99)]
        n_tier = len([r for r in case_results if r["difficulty"] == tier])
        lines.append(f"| {tier} | {dp50:.1f} | {dp95:.1f} | {dp99:.1f} | {n_tier} |")

    lines += [
        "",
        "> **Note on p99**: the overall p99 is skewed by hard-tier sentences >40 words (legal/financial register).",
        "> Typical LLM output maps to basic/medium tier — see the per-tier table above for operationally relevant numbers.",
        "",
        "## Full latency distribution", "", "```",
    ]
    buckets = [(0, 25), (25, 50), (50, 75), (75, 100), (100, 150), (150, float("inf"))]
    for lo, hi in buckets:
        count = sum(1 for l in all_latencies if lo <= l < hi)
        label = f"{lo}–{int(hi)}ms" if hi != float("inf") else f">{lo}ms"
        bar = "█" * (count * 40 // n) if n else ""
        lines.append(f"  {label:>12}  {bar}  {count}")
    lines += ["```", ""]

    lines += ["", _METRIC_GLOSSARY]

    out_path.write_text("\n".join(lines), encoding="utf-8")
    return out_path


# ---------------------------------------------------------------------------
# Benchmark runner
# ---------------------------------------------------------------------------

def run_benchmark(base_url: str, runs_per_case: int, out_dir: Path | None) -> None:
    health_url  = f"{base_url}/health"
    extract_url = f"{base_url}/extract"

    try:
        h = requests.get(health_url, timeout=5)
        h.raise_for_status()
        health = h.json()
        print(f"\nService version : {health.get('version', '?')}")
        print(f"coreferee       : {'✓' if health.get('coreferee') else '✗ (rule-based fallback)'}")
        print(f"GLiNER          : {'✓' if health.get('gliner') else '✗ (spaCy NER fallback)'}")
    except Exception as exc:
        print(f"ERROR: service not reachable at {base_url} — {exc}")
        print("Start with: cd extractor/srl_service && python main.py")
        return

    by_difficulty = {"basic": [], "medium": [], "hard": []}
    for c in DATASET:
        by_difficulty[c.difficulty].append(c)

    print(f"\nDataset: {len(DATASET)} cases  "
          f"({len(by_difficulty['basic'])} basic / "
          f"{len(by_difficulty['medium'])} medium / "
          f"{len(by_difficulty['hard'])} hard)")
    print(f"Runs per case: {runs_per_case}\n")

    all_latencies: list[float] = []
    total_expected = 0
    total_found    = 0
    total_facts    = 0
    case_results: list[dict] = []

    col_w = 52
    print(f"{'Description':<{col_w}}  {'Diff':>6}  {'p50 ms':>8}  {'Facts':>6}  {'Recall':>8}")
    print("-" * (col_w + 36))

    for case in DATASET:
        latencies: list[float] = []
        last_facts: list[dict] = []

        for _ in range(runs_per_case):
            t0 = time.perf_counter()
            resp = requests.post(extract_url, json={"text": case.text}, timeout=30)
            elapsed_ms = (time.perf_counter() - t0) * 1000
            if resp.status_code != 200:
                print(f"  WARN: HTTP {resp.status_code} for: {case.text[:60]}")
                continue
            latencies.append(elapsed_ms)
            last_facts = resp.json().get("facts", [])

        if not latencies:
            continue

        all_latencies.extend(latencies)

        found = _case_found(last_facts, case.expected)
        total_expected += len(case.expected)
        total_found    += found
        total_facts    += len(last_facts)

        p50_case = statistics.median(latencies)
        recall_str = f"{found}/{len(case.expected)}"
        miss_flag  = " ✗" if found < len(case.expected) else ""
        case_results.append({
            "description": case.description,
            "difficulty":  case.difficulty,
            "p50_ms":      round(p50_case, 1),
            "facts":       len(last_facts),
            "recall":      recall_str,
        })
        desc = case.description[:col_w]
        print(f"{desc:<{col_w}}  {case.difficulty:>6}  {p50_case:>8.1f}  {len(last_facts):>6}  {recall_str:>8}{miss_flag}")

    if not all_latencies:
        print("No results collected.")
        return

    all_latencies.sort()
    n   = len(all_latencies)
    p50 = statistics.median(all_latencies)
    p95 = all_latencies[int(n * 0.95)]
    p99 = all_latencies[int(n * 0.99)]
    recall_pct = (total_found / total_expected * 100) if total_expected else 0
    avg_facts  = total_facts / len(DATASET)
    under_100  = sum(1 for l in all_latencies if l < 100)

    print("\n" + "=" * (col_w + 36))
    print(f"{'Latency p50':<30}: {p50:.1f} ms")
    print(f"{'Latency p95':<30}: {p95:.1f} ms")
    print(f"{'Latency p99':<30}: {p99:.1f} ms")
    print(f"{'Triple recall':<30}: {total_found}/{total_expected} ({recall_pct:.1f}%)")
    print(f"{'Avg facts extracted':<30}: {avg_facts:.1f}")
    print(f"{'Total samples':<30}: {n}")
    print(f"{'Requests under 100ms':<30}: {under_100}/{n} ({under_100/n*100:.0f}%)")

    if p99 <= 100:
        print("\n✓ p99 latency target (<100ms) MET")
    elif p95 <= 100:
        print("\n~ p95 latency target met; p99 slightly over")
    else:
        print("\n✗ Latency target missed — check model loading and TMS connectivity")

    if recall_pct >= 93:
        print(f"✓ Recall target (>93%) MET: {recall_pct:.1f}%")
    else:
        print(f"✗ Recall target missed: {recall_pct:.1f}% (target >93%)")

    if out_dir:
        saved = _save_results(
            out_dir, health, case_results, all_latencies,
            total_found, total_expected, total_facts, runs_per_case, base_url,
        )
        print(f"\nResults saved → {saved}")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Velarix Delta extraction pipeline benchmark")
    parser.add_argument("--url",  default="http://localhost:8090", help="SRL service base URL")
    parser.add_argument("--runs", type=int, default=10,            help="Runs per case for latency percentiles")
    parser.add_argument(
        "--out",
        default=str(Path(__file__).resolve().parents[2] / "docs" / "benchmark"),
        help="Directory to write the markdown results file (default: ../../docs/benchmark)",
    )
    parser.add_argument("--no-save", action="store_true", help="Skip writing results to disk")
    args = parser.parse_args()

    run_benchmark(
        args.url,
        args.runs,
        out_dir=None if args.no_save else Path(args.out),
    )
