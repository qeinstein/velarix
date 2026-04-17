"""
Delta dependency benchmark — Velarix fact extraction pipeline.

Measures how accurately stage 5 infers which facts depend on which.
Complements benchmark.py (which measures triple recall and latency).

Each test case specifies:
  - input text
  - expected dependency edges as (parent_hint, child_hint, relation_type)
    where each hint is a (subject_hint, predicate_hint, object_hint) tuple
    matched by substring against the extracted facts

Metrics reported:
  - Edge precision: of the edges Delta proposed, what fraction are correct?
  - Edge recall:    of the expected edges, what fraction did Delta find?
  - F1:            harmonic mean of precision and recall
  - Per-mechanism breakdown: connective / entity_overlap / chain / coref
  - False positives: edges proposed that have no ground-truth match
  - False negatives: expected edges Delta missed entirely

Usage:
    # Start Delta first:
    #   cd extractor/srl_service && python3 main.py
    #
    # Then run:
    #   python3 dependency_benchmark.py
    #   python3 dependency_benchmark.py --url http://localhost:8090 --out ../../docs/benchmark
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
# Each DependencyCase has:
#   text          — input to POST /extract
#   edges         — list of ExpectedEdge
#   description   — human label
#   mechanism     — primary mechanism expected to fire (connective/overlap/chain/coref)
#
# ExpectedEdge fields:
#   parent_hint   — (subj, pred, obj) substrings identifying the parent fact
#   child_hint    — (subj, pred, obj) substrings identifying the child fact
#   relation      — expected edge type (causal/temporal/contrastive/conditional/
#                   entity_overlap/chain/coreference) — used for per-type breakdown
#   required      — if False, the edge is "nice to have" but not counted as a miss
# ---------------------------------------------------------------------------

class EdgeHint(NamedTuple):
    subject: str
    predicate: str
    object: str


class ExpectedEdge(NamedTuple):
    parent_hint: EdgeHint
    child_hint: EdgeHint
    relation: str
    required: bool = True


class DependencyCase(NamedTuple):
    text: str
    edges: list[ExpectedEdge]
    description: str
    mechanism: str  # primary mechanism tag for breakdown table


DATASET: list[DependencyCase] = [

    # =========================================================================
    # CONNECTIVE EDGES — discourse markers create typed directed edges
    # =========================================================================

    DependencyCase(
        "The vendor failed to deliver. The contract was therefore terminated.",
        [ExpectedEdge(
            EdgeHint("vendor", "fail", ""),
            EdgeHint("contract", "terminat", ""),
            "causal",
        )],
        "causal: therefore links vendor-fail → contract-terminated",
        "connective",
    ),
    DependencyCase(
        "The payment was delayed because the invoice was disputed.",
        [ExpectedEdge(
            EdgeHint("invoice", "disput", ""),
            EdgeHint("payment", "delay", ""),
            "causal",
        )],
        "causal: because clause — invoice disputed → payment delayed",
        "connective",
    ),
    DependencyCase(
        "The merger was approved. Consequently, the shareholders received dividends.",
        [ExpectedEdge(
            EdgeHint("merger", "approv", ""),
            EdgeHint("shareholder", "receiv", "dividend"),
            "causal",
        )],
        "causal: consequently links merger-approved → shareholders-received",
        "connective",
    ),
    DependencyCase(
        "Although the payment was delayed, the vendor fulfilled the contract.",
        [ExpectedEdge(
            EdgeHint("payment", "delay", ""),
            EdgeHint("vendor", "fulfil", "contract"),
            "contrastive",
        )],
        "contrastive: although — payment delayed, vendor fulfilled",
        "connective",
    ),
    DependencyCase(
        "The board approved the loan after the due diligence report was submitted.",
        [ExpectedEdge(
            EdgeHint("report", "submit", ""),
            EdgeHint("board", "approv", "loan"),
            "temporal",
        )],
        "temporal: after — report submitted → board approved",
        "connective",
    ),
    DependencyCase(
        "Following the acquisition, the firm expanded into new markets.",
        [ExpectedEdge(
            EdgeHint("firm", "expand", ""),
            EdgeHint("firm", "expand", ""),
            "temporal",
            False,  # the acquisition may not extract cleanly; mark optional
        )],
        "temporal: following — acquisition → expansion (optional, participial)",
        "connective",
    ),
    DependencyCase(
        "The deal will proceed provided that the regulators approve it.",
        [ExpectedEdge(
            EdgeHint("regulator", "approv", ""),
            EdgeHint("deal", "proceed", ""),
            "conditional",
        )],
        "conditional: provided that — regulator approval gates deal",
        "connective",
    ),
    DependencyCase(
        "The system was shut down because the audit revealed a critical vulnerability.",
        [ExpectedEdge(
            EdgeHint("audit", "reveal", ""),
            EdgeHint("system", "shut", ""),
            "causal",
        )],
        "causal: because — audit-revealed → system-shut-down",
        "connective",
    ),
    DependencyCase(
        "The contract was signed. Subsequently, the project commenced.",
        [ExpectedEdge(
            EdgeHint("contract", "sign", ""),
            EdgeHint("project", "commenc", ""),
            "temporal",
        )],
        "temporal: subsequently — contract signed → project commenced",
        "connective",
    ),
    DependencyCase(
        "The fine was issued. However, the bank appealed the decision.",
        [ExpectedEdge(
            EdgeHint("fine", "issu", ""),
            EdgeHint("bank", "appeal", ""),
            "contrastive",
        )],
        "contrastive: however — fine issued, bank appealed",
        "connective",
    ),

    # =========================================================================
    # ENTITY OVERLAP EDGES — shared entity strings link related facts
    # =========================================================================

    DependencyCase(
        "Apple acquired GitHub. GitHub launched Copilot.",
        [ExpectedEdge(
            EdgeHint("apple", "acquir", "github"),
            EdgeHint("github", "launch", "copilot"),
            "entity_overlap",
        )],
        "overlap: GitHub appears in both facts",
        "entity_overlap",
    ),
    DependencyCase(
        "The board approved the merger. The board also approved the restructuring plan.",
        [ExpectedEdge(
            EdgeHint("board", "approv", "merger"),
            EdgeHint("board", "approv", "restructur"),
            "entity_overlap",
        )],
        "overlap: same subject (board) in two facts",
        "entity_overlap",
    ),
    DependencyCase(
        "Alice signed the contract. The contract was submitted to the regulator.",
        [ExpectedEdge(
            EdgeHint("alice", "sign", "contract"),
            EdgeHint("contract", "submit", ""),
            "entity_overlap",
        )],
        "overlap: contract appears as object then subject",
        "entity_overlap",
    ),
    DependencyCase(
        "The regulator fined the bank. The bank disputed the fine.",
        [ExpectedEdge(
            EdgeHint("regulator", "fin", "bank"),
            EdgeHint("bank", "disput", "fine"),
            "entity_overlap",
        )],
        "overlap: bank shared across both facts",
        "entity_overlap",
    ),
    DependencyCase(
        "The vendor delivered the goods. The goods passed inspection.",
        [ExpectedEdge(
            EdgeHint("vendor", "deliver", "goods"),
            EdgeHint("goods", "pass", "inspection"),
            "entity_overlap",
        )],
        "overlap: goods as object then subject (also chain candidate)",
        "entity_overlap",
    ),

    # =========================================================================
    # CHAIN EDGES — fact A's object is fact B's subject
    # =========================================================================

    DependencyCase(
        "Microsoft acquired Activision. Activision released a new title.",
        [ExpectedEdge(
            EdgeHint("microsoft", "acquir", "activision"),
            EdgeHint("activision", "releas", ""),
            "chain",
        )],
        "chain: Activision is object of acquisition, subject of release",
        "chain",
    ),
    DependencyCase(
        "The committee approved the proposal. The proposal was implemented immediately.",
        [ExpectedEdge(
            EdgeHint("committee", "approv", "proposal"),
            EdgeHint("proposal", "implement", ""),
            "chain",
        )],
        "chain: proposal as object → subject",
        "chain",
    ),
    DependencyCase(
        "The auditor identified the fraud. The fraud triggered an investigation.",
        [ExpectedEdge(
            EdgeHint("auditor", "identif", "fraud"),
            EdgeHint("fraud", "trigger", "investigation"),
            "chain",
        )],
        "chain: fraud as object → subject (causal chain)",
        "chain",
    ),
    DependencyCase(
        "The judge issued the injunction. The injunction halted the merger.",
        [ExpectedEdge(
            EdgeHint("judge", "issu", "injunction"),
            EdgeHint("injunction", "halt", "merger"),
            "chain",
        )],
        "chain: injunction object → subject",
        "chain",
    ),
    DependencyCase(
        "The engineer filed the patent. The patent protected the invention.",
        [ExpectedEdge(
            EdgeHint("engineer", "fil", "patent"),
            EdgeHint("patent", "protect", "invention"),
            "chain",
        )],
        "chain: patent object → subject",
        "chain",
    ),

    # =========================================================================
    # COREFERENCE EDGES — resolved pronouns generate dependency edges
    # =========================================================================

    DependencyCase(
        "Alice submitted the report. She included all required sections.",
        [ExpectedEdge(
            EdgeHint("alice", "submit", "report"),
            EdgeHint("alice", "includ", "sections"),
            "coreference",
        )],
        "coref: she → Alice; alice-submit is parent of alice-included",
        "coreference",
    ),
    DependencyCase(
        "The board reviewed the proposal. They rejected it unanimously.",
        [ExpectedEdge(
            EdgeHint("board", "review", "proposal"),
            EdgeHint("board", "reject", "proposal"),
            "coreference",
        )],
        "coref: they → board, it → proposal",
        "coreference",
    ),
    DependencyCase(
        "The CEO announced the restructuring. He resigned the following week.",
        [ExpectedEdge(
            EdgeHint("ceo", "announc", "restructur"),
            EdgeHint("ceo", "resign", ""),
            "coreference",
        )],
        "coref: he → CEO; announce is parent of resigned",
        "coreference",
    ),
    DependencyCase(
        "The manager approved the budget. She then notified the team.",
        [ExpectedEdge(
            EdgeHint("manager", "approv", "budget"),
            EdgeHint("manager", "notif", "team"),
            "coreference",
        )],
        "coref: she → manager",
        "coreference",
    ),

    # =========================================================================
    # NEGATIVE CASES — no dependency should be inferred
    # (these test precision — the pipeline should NOT create edges here)
    # =========================================================================

    DependencyCase(
        "Tesla makes electric cars. The weather is sunny today.",
        [],  # no expected edges — unrelated facts
        "negative: completely unrelated facts should produce no edges",
        "none",
    ),
    DependencyCase(
        "The contract is void. The payment remains pending.",
        [],
        "negative: two independent state facts with no shared entity",
        "none",
    ),
]


# ---------------------------------------------------------------------------
# Fact matching
# ---------------------------------------------------------------------------

def _normalize(s: str) -> str:
    return s.lower().strip()


def _fact_matches_hint(fact: dict, hint: EdgeHint) -> bool:
    subj = _normalize(fact.get("subject", ""))
    pred = _normalize(fact.get("predicate", ""))
    obj  = _normalize(fact.get("object",   ""))
    if hint.subject   and hint.subject   not in subj: return False
    if hint.predicate and hint.predicate not in pred: return False
    if hint.object    and hint.object    not in obj:  return False
    return True


def _find_fact(facts: list[dict], hint: EdgeHint) -> dict | None:
    for f in facts:
        if _fact_matches_hint(f, hint):
            return f
    return None


def _edge_exists(facts: list[dict], parent_id: str, child_id: str) -> bool:
    """Return True if child fact has parent_id in its depends_on list."""
    for f in facts:
        if f["id"] == child_id and parent_id in f.get("depends_on", []):
            return True
    return False


# ---------------------------------------------------------------------------
# Report output
# ---------------------------------------------------------------------------

def _save_results(
    out_dir: Path,
    health: dict,
    case_results: list[dict],
    global_stats: dict,
    base_url: str,
) -> Path:
    out_dir.mkdir(parents=True, exist_ok=True)
    ts = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    coref_tag = "neural-coref" if health.get("coreferee") else "rule-coref"
    ner_tag   = "gliner-ner"   if health.get("gliner")    else "spacy-ner"
    ver       = health.get("version", "?")

    prec = global_stats["precision"]
    rec  = global_stats["recall"]
    f1   = global_stats["f1"]

    fname = (
        f"delta_dep_{coref_tag}_{ner_tag}"
        f"_prec-{prec:.0%}_rec-{rec:.0%}_f1-{f1:.0%}"
        f"_{ts}.md"
    )
    out_path = out_dir / fname

    try:
        git_sha = subprocess.check_output(
            ["git", "rev-parse", "--short", "HEAD"], stderr=subprocess.DEVNULL
        ).decode().strip()
    except Exception:
        git_sha = "unknown"

    lines: list[str] = [
        f"# Delta Dependency Benchmark — {ts}",
        "",
        "Measures how accurately stage 5 infers `depends_on` edges between facts.",
        "Complements the triple-recall benchmark which measures stage 4 extraction.",
        "",
        "## Run configuration",
        "",
        "| Key | Value |",
        "|-----|-------|",
        f"| Date (UTC) | {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S')} |",
        f"| Git SHA | `{git_sha}` |",
        f"| Service URL | {base_url} |",
        f"| Service version | {ver} |",
        f"| coreferee | {'✓ neural' if health.get('coreferee') else '✗ rule-based fallback'} |",
        f"| GLiNER | {'✓ active' if health.get('gliner') else '✗ spaCy NER fallback'} |",
        f"| Python | {sys.version.split()[0]} |",
        f"| Platform | {platform.platform()} |",
        "",
        "## Summary",
        "",
        "| Metric | Result | Target |",
        "|--------|--------|--------|",
        f"| Edge precision | {prec:.1%} | >70% |",
        f"| Edge recall    | {rec:.1%} | >70% |",
        f"| F1             | {f1:.1%} | >70% |",
        f"| True positives | {global_stats['tp']} | — |",
        f"| False positives | {global_stats['fp']} | — |",
        f"| False negatives | {global_stats['fn']} | — |",
        f"| Negative cases correct (no spurious edge) | {global_stats['tn_correct']}/{global_stats['tn_total']} | — |",
        "",
        "## Per-mechanism breakdown",
        "",
        "| Mechanism | TP | FP | FN | Precision | Recall | F1 |",
        "|-----------|----|----|----|-----------+--------|-----|",
    ]

    for mech, ms in global_stats["by_mechanism"].items():
        mtp = ms["tp"]; mfp = ms["fp"]; mfn = ms["fn"]
        mp = mtp / (mtp + mfp) if (mtp + mfp) else 0.0
        mr = mtp / (mtp + mfn) if (mtp + mfn) else 0.0
        mf = 2 * mp * mr / (mp + mr) if (mp + mr) else 0.0
        lines.append(f"| {mech} | {mtp} | {mfp} | {mfn} | {mp:.1%} | {mr:.1%} | {mf:.1%} |")

    lines += [
        "",
        "## Per-case results",
        "",
        "| # | Mechanism | Description | Expected edges | TP | FP | FN | Result |",
        "|---|-----------|-------------|----------------|----|----|-----|--------|",
    ]
    for i, r in enumerate(case_results, 1):
        verdict = "✓" if r["fn"] == 0 and r["fp"] == 0 else ("~ FP" if r["fn"] == 0 else ("~ FN" if r["fp"] == 0 else "✗"))
        lines.append(
            f"| {i} | {r['mechanism']} | {r['description']} "
            f"| {r['expected']} | {r['tp']} | {r['fp']} | {r['fn']} | {verdict} |"
        )

    lines += ["", "## False positives (spurious edges)", ""]
    fps = [r for r in case_results if r["fp"] > 0]
    if fps:
        for r in fps:
            lines.append(f"- **{r['description']}**: {r['fp']} spurious edge(s) — {r['fp_detail']}")
    else:
        lines.append("None.")

    lines += ["", "## False negatives (missed edges)", ""]
    fns = [r for r in case_results if r["fn"] > 0]
    if fns:
        for r in fns:
            lines.append(f"- **{r['description']}**: {r['fn']} edge(s) missed — {r['fn_detail']}")
    else:
        lines.append("None.")

    lines += [
        "",
        "## Metric glossary",
        "",
        "| Metric | Meaning |",
        "|--------|---------|",
        "| **Edge precision** | Of all dependency edges Delta proposed, what fraction are in the ground truth? High precision means few spurious links. |",
        "| **Edge recall** | Of all expected dependency edges, what fraction did Delta find? High recall means few missed links. |",
        "| **F1** | Harmonic mean of precision and recall. The primary headline metric for dependency accuracy. |",
        "| **TP** | True positive — an expected edge that Delta correctly produced. |",
        "| **FP** | False positive — an edge Delta produced that is not in the ground truth. |",
        "| **FN** | False negative — an expected edge that Delta missed entirely. |",
        "| **Negative case** | A test case with no expected edges. Checks that the pipeline does not invent spurious links between unrelated facts. |",
        "",
        "### Mechanisms",
        "",
        "| Mechanism | Source | Confidence |",
        "|-----------|--------|------------|",
        "| connective | Discourse markers (because, although, after, therefore…) | 0.90 |",
        "| entity_overlap | Shared entity string between two facts | 0.45–0.88 (entity-type weighted) |",
        "| chain | Fact A's object == Fact B's subject (object→subject chain) | 0.80 |",
        "| coreference | Resolved coreference chain (she → Alice) | inherited from coref model |",
    ]

    out_path.write_text("\n".join(lines), encoding="utf-8")
    return out_path


# ---------------------------------------------------------------------------
# Benchmark runner
# ---------------------------------------------------------------------------

_REL_TO_MECH: dict[str, str] = {
    "causal":        "connective",
    "temporal":      "connective",
    "conditional":   "connective",
    "contrastive":   "connective",
    "entity_overlap":"entity_overlap",
    "chain":         "chain",
    "coreference":   "coreference",
}


def run_benchmark(base_url: str, out_dir: Path | None) -> None:
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
        print("Start with: cd extractor/srl_service && python3 main.py")
        return

    by_mech: dict[str, dict] = {}
    for mech in ("connective", "entity_overlap", "chain", "coreference"):
        by_mech[mech] = {"tp": 0, "fp": 0, "fn": 0}

    global_tp = global_fp = global_fn = 0
    tn_correct = tn_total = 0

    case_results: list[dict] = []

    col_w = 55
    print(f"\n{'Description':<{col_w}}  {'Mech':>12}  {'Exp':>4}  {'TP':>3}  {'FP':>3}  {'FN':>3}")
    print("-" * (col_w + 30))

    for case in DATASET:
        try:
            resp = requests.post(extract_url, json={"text": case.text}, timeout=30)
            resp.raise_for_status()
        except Exception as exc:
            print(f"  WARN: request failed for '{case.text[:50]}': {exc}")
            continue

        facts: list[dict] = resp.json().get("facts", [])

        required_edges = [e for e in case.edges if e.required]

        is_negative = len(required_edges) == 0

        tp = fp = fn = 0
        fp_details: list[str] = []
        fn_details: list[str] = []

        if is_negative:
            tn_total += 1
            # Check: any fact has depends_on entries → spurious edge
            has_spurious = any(f.get("depends_on") for f in facts)
            if not has_spurious:
                tn_correct += 1
                verdict = "✓ (no spurious edge)"
            else:
                fp += sum(len(f.get("depends_on", [])) for f in facts)
                global_fp += fp
                by_mech["entity_overlap"]["fp"] += fp  # unrelated facts usually come via overlap
                fp_details.append(f"spurious depends_on found on {fp} fact(s)")
                by_mech["entity_overlap"]["fp"] += fp  # negative cases: noise from overlap
                verdict = f"✗ ({fp} spurious edge(s))"
        else:
            # Check each required expected edge
            for expected_edge in required_edges:
                parent_fact = _find_fact(facts, expected_edge.parent_hint)
                child_fact  = _find_fact(facts, expected_edge.child_hint)

                if parent_fact is None or child_fact is None:
                    # Can't evaluate — the fact itself wasn't extracted
                    fn += 1
                    fn_details.append(
                        f"fact not extracted: parent={expected_edge.parent_hint}, "
                        f"child={expected_edge.child_hint}"
                    )
                    rel = expected_edge.relation.split("_")[0] if "_" in expected_edge.relation else expected_edge.relation
                    mech_key = _REL_TO_MECH.get(expected_edge.relation, "entity_overlap")
                    by_mech[mech_key]["fn"] += 1
                    continue

                if _edge_exists(facts, parent_fact["id"], child_fact["id"]):
                    tp += 1
                    mech_key = _REL_TO_MECH.get(expected_edge.relation, "entity_overlap")
                    by_mech[mech_key]["tp"] += 1
                else:
                    fn += 1
                    fn_details.append(
                        f"edge missing: '{parent_fact['subject']} {parent_fact['predicate']}' "
                        f"→ '{child_fact['subject']} {child_fact['predicate']}'"
                    )
                    mech_key = _REL_TO_MECH.get(expected_edge.relation, "entity_overlap")
                    by_mech[mech_key]["fn"] += 1

            # Count false positives: edges on facts in this case that have no GT match
            all_proposed_pairs: set[tuple[str, str]] = set()
            for f in facts:
                for parent_id in f.get("depends_on", []):
                    all_proposed_pairs.add((parent_id, f["id"]))

            gt_pairs: set[tuple[str, str]] = set()
            for expected_edge in required_edges:
                pf = _find_fact(facts, expected_edge.parent_hint)
                cf = _find_fact(facts, expected_edge.child_hint)
                if pf and cf:
                    gt_pairs.add((pf["id"], cf["id"]))

            spurious = all_proposed_pairs - gt_pairs
            fp += len(spurious)
            for pid, cid in spurious:
                pf_text = next((f"{f['subject']} {f['predicate']}" for f in facts if f["id"] == pid), pid[:12])
                cf_text = next((f"{f['subject']} {f['predicate']}" for f in facts if f["id"] == cid), cid[:12])
                fp_details.append(f"'{pf_text}' → '{cf_text}'")
                # Edge source not preserved in depends_on; attribute FP to case mechanism.
                mech_key = _REL_TO_MECH.get(case.mechanism, "entity_overlap")
                by_mech[mech_key]["fp"] += 1

            global_tp += tp
            global_fp += fp
            global_fn += fn

        case_results.append({
            "description": case.description,
            "mechanism":   case.mechanism,
            "expected":    len(required_edges),
            "tp": tp, "fp": fp, "fn": fn,
            "fp_detail": "; ".join(fp_details) if fp_details else "—",
            "fn_detail": "; ".join(fn_details) if fn_details else "—",
        })

        desc = case.description[:col_w]
        print(f"{desc:<{col_w}}  {case.mechanism:>12}  {len(required_edges):>4}  {tp:>3}  {fp:>3}  {fn:>3}")

    precision = global_tp / (global_tp + global_fp) if (global_tp + global_fp) else 0.0
    recall    = global_tp / (global_tp + global_fn) if (global_tp + global_fn) else 0.0
    f1        = 2 * precision * recall / (precision + recall) if (precision + recall) else 0.0

    global_stats = {
        "tp": global_tp, "fp": global_fp, "fn": global_fn,
        "precision": precision, "recall": recall, "f1": f1,
        "tn_correct": tn_correct, "tn_total": tn_total,
        "by_mechanism": by_mech,
    }

    print("\n" + "=" * (col_w + 30))
    print(f"{'Edge precision':<30}: {precision:.1%}  (target >70%)")
    print(f"{'Edge recall':<30}: {recall:.1%}  (target >70%)")
    print(f"{'F1':<30}: {f1:.1%}")
    print(f"{'True positives':<30}: {global_tp}")
    print(f"{'False positives':<30}: {global_fp}")
    print(f"{'False negatives':<30}: {global_fn}")
    print(f"{'Negative cases correct':<30}: {tn_correct}/{tn_total}")

    print("\nPer-mechanism:")
    for mech, ms in by_mech.items():
        mtp = ms["tp"]; mfp = ms["fp"]; mfn = ms["fn"]
        mp = mtp / (mtp + mfp) if (mtp + mfp) else 0.0
        mr = mtp / (mtp + mfn) if (mtp + mfn) else 0.0
        mf = 2 * mp * mr / (mp + mr) if (mp + mr) else 0.0
        print(f"  {mech:<18} prec={mp:.1%}  rec={mr:.1%}  f1={mf:.1%}  (tp={mtp} fp={mfp} fn={mfn})")

    if f1 >= 0.70:
        print(f"\n✓ Dependency F1 target (>70%) MET: {f1:.1%}")
    else:
        print(f"\n✗ Dependency F1 target missed: {f1:.1%} (target >70%)")

    if out_dir:
        saved = _save_results(out_dir, health, case_results, global_stats, base_url)
        print(f"\nResults saved → {saved}")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Delta dependency accuracy benchmark"
    )
    parser.add_argument("--url", default="http://localhost:8090", help="SRL service base URL")
    parser.add_argument(
        "--out",
        default=str(Path(__file__).resolve().parents[2] / "docs" / "benchmark"),
        help="Directory to write the markdown results file",
    )
    parser.add_argument("--no-save", action="store_true", help="Skip writing results to disk")
    args = parser.parse_args()

    run_benchmark(
        args.url,
        out_dir=None if args.no_save else Path(args.out),
    )
