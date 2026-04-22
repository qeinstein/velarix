# Velarix Extraction Pipeline — Finance Wedge

## Scope

This document defines what to extract, when to extract it, and the exact pipeline
for both sources. All decisions are scoped to the finance domain. Finance narrows
the fact space enough that a grammar-based parser can cover the majority of
assertion patterns with high precision and determinism — no LLM in the extraction
hot path.

---

## What is a Fact in Finance

Every fact that enters the TMS must be a **Subject → Predicate → Object** triple
with a confidence score and assertion kind. In finance, the universe of meaningful
facts is finite and recognizable:

| Category | Example triple |
|---|---|
| Amount | `(invoice-1042, amount, $5000)` |
| Status | `(vendor-17, status, verified)` |
| Approval | `(user:alice, approved, payment-1042)` |
| Compliance event | `(vendor-17, passed, KYC)` |
| Threshold breach | `(transaction-99, exceeds, AML-threshold-10000)` |
| Relationship | `(vendor-17, controlled-by, entity-X)` |
| Block/hold | `(payment-1042, blocked-by, compliance-review)` |
| Date/expiry | `(kyc-cert-vendor-17, expires, 2026-12-31)` |

If an assertion does not fit one of these categories, it is not a fact for Velarix.
It gets discarded, not quarantined. This is the core benefit of the finance wedge:
you know ahead of time what you're looking for.

---

## Source 1: User Messages

### When to extract

Run the classifier on every user message before any NLP. Skip anything that is
not a declarative assertion.

**Extract:**
- Explicit status assertions: "Vendor 17 is verified", "The invoice is approved"
- Approvals: "I approve this payment", "I authorize the transfer"
- Fact corrections: "No, the amount is $6000" (extract corrected value, flag prior)
- Event completions: "The vendor passed KYC", "The audit failed"
- Amount/threshold assertions: "The invoice is for $50,000"
- Relationship assertions: "Vendor 17 is owned by Acme Corp"

**Do not extract:**
- Questions: "Is the vendor verified?", "What is the invoice amount?"
- Commands without assertion: "Release the payment", "Block the transfer"
- Acknowledgements: "Okay", "Got it", "Understood", "Thanks"
- Conditionals without assertion: "If the vendor passes KYC, we can proceed"
- Hedged statements from users: "I think the amount might be $5000" → quarantine, do not assert

### What becomes a root fact

User assertions become **root facts** (`is_root: true`) with confidence `0.7` by
default. User assertions are ground truth from a human — they are the premises
everything else depends on. A user saying "I approve this payment" is a root fact.
The LLM's downstream recommendation ("payment should be released") is a derived
fact that depends on it.

### Classification rules (Go, ~1ms)

Applied in order, first match wins:

```
1. Ends with "?" → QUESTION → skip
2. Starts with interrogative word (who/what/when/where/why/how) + no comma before verb → QUESTION → skip
3. Starts with imperative verb (release/block/transfer/send/check/verify/approve/reject)
   with no explicit subject → COMMAND → skip
4. Is ≤ 3 words with no financial entity → CONVERSATIONAL → skip
5. Contains correction marker ("no,", "actually,", "correction:") → CORRECTION → extract with flag
6. Contains approval verb (approve/authorize/sign off/confirm) with first-person subject → APPROVAL → extract
7. Contains financial predicate (see predicate list below) → ASSERTION → extract
8. Contains financial entity (amount, account number, vendor ID, date) → ASSERTION → extract
9. Otherwise → skip
```

---

## Source 2: LLM Output

### When to extract

**Controlled LLM (system prompt accessible):** Always. The LLM emits a structured
`<FACTS>` block as part of every response. Parse it deterministically. No NLP.

**Uncontrolled LLM (no system prompt access):** Extract from every response that
contains declarative financial content. Skip responses that are purely
conversational ("I'll help you with that") or purely questions back to the user.

### What becomes a derived fact

LLM output contains two types of assertions:

1. **LLM analysis of facts the user stated** — these should reference back to
   user-asserted root facts. They become derived facts with `depends_on` pointing
   to the root. Example: "Based on the verified KYC, payment is cleared" →
   derived fact depending on `(vendor-17, passed, KYC)`.

2. **LLM-discovered facts from external data** — when the LLM has access to
   external tools (database lookups, document parsing), its factual output is a
   root fact. Example: LLM calls a KYB API and reports "Vendor 17 passed KYB" →
   root fact, source: `llm_tool_output`, confidence `0.85`.

The distinction matters for the execute-check: derived facts are only as valid as
their roots. LLM-discovered root facts are direct premises.

---

## The Extraction Pipeline

### Controlled LLM path (primary, target: <10ms)

**Why production-ready:** Parsing structured JSON is deterministic. The LLM is
already running — this adds zero model overhead. The only failure mode is the LLM
not emitting the FACTS block, which is caught by the parser and falls back to the
uncontrolled path.

**Step 1 — System prompt injection**

Add to every LLM system prompt:

```
After your response, output a FACTS block using this exact format:

<FACTS>
[
  {
    "id": "slug-string",
    "subject": "entity name or ID",
    "predicate": "one of the finance predicates",
    "object": "value, entity, or null",
    "confidence": 0.0-1.0,
    "kind": "empirical|uncertain|hypothetical",
    "is_root": true|false,
    "depends_on": ["fact-id-1", "fact-id-2"],
    "source": "llm_analysis|llm_tool_output|user_stated"
  }
]
</FACTS>

Rules:
- Only emit facts that are explicitly present in the conversation. Do not infer.
- For user_stated facts, the source is "user_stated" and is_root is true.
- For your own analysis, source is "llm_analysis" and is_root is false.
- depends_on must reference IDs from the active facts list below or IDs you emit earlier in this block.
- If you have no facts to assert, emit <FACTS>[]</FACTS>.

ACTIVE FACTS (current session graph, top 20 by recency):
{active_facts_json}
```

The `{active_facts_json}` is injected by the Go server when building the prompt.
It contains the current session's fact IDs and claims. This is how the LLM knows
which node IDs to reference in `depends_on` — it sees the live graph.

**Step 2 — Go parser (~1ms)**

Split the LLM response at `<FACTS>` and `</FACTS>`. Parse the JSON array. Validate
each fact against the finance fact schema (predicate must be in the known predicate
list, confidence must be 0.0-1.0, id must be a valid slug).

**Step 3 — Graph validation (~2ms)**

Feed each fact into the existing TMS consistency precheck. Contradictions go to the
contradiction queue. Accepted facts are asserted in topological order (roots first).

**Tradeoff:** LLM occasionally omits facts from the FACTS block or emits a
malformed block.
**Solution:** On parse failure, fall back to the uncontrolled path automatically.
Log the failure. After 3 consecutive parse failures on the same session, surface an
alert — it means the model is not following the system prompt, which needs
investigation.

---

### User message path (target: <40ms)

**Why production-ready:** Every component is deterministic, maintained, and has
known failure modes. No external dependencies beyond the Python sidecar (already
in the architecture). The grammar-based parser for finance is exhaustive over the
known predicate space — it does not attempt to handle arbitrary English.

**Step 1 — Classifier (Go, ~1ms)**

Apply the classification rules from the "When to extract" section above. Written
in Go using regex and token matching. Returns: `skip | assertion | approval | correction`.
Only `assertion`, `approval`, and `correction` proceed.

**Step 2 — Financial entity extraction (Go, ~1ms)**

Before NLP, extract typed financial entities using regex. These are the
anchors for the triple.

Entities extracted by pattern:
- **Amounts:** `\$[\d,]+(\.\d{2})?` → `currency_amount`
- **Percentages:** `\d+(\.\d+)?\s*%` → `percentage`
- **Account/invoice IDs:** configurable per deployment (e.g. `INV-\d+`, `ACC-\d+`)
- **Dates:** ISO 8601 + common formats (`March 15`, `Q3 2026`)
- **Named entities:** vendor names, person names — covered by spaCy NER in Step 3

This step runs in Go with no external call. It pre-populates the subject and object
candidates so the parser in Step 3 has typed anchors, not raw strings.

**Step 3 — Grammar-based assertion parser (Go, ~2ms)**

A hand-written parser covering the finance assertion grammar. For the finance domain,
this covers the vast majority of user assertions because financial language is formal
and follows conventions.

Pattern groups (applied in order):

```
APPROVAL:
  "[I|we] [approve|authorize|sign off on|confirm] [NP]"
  → (subject: actor_id, predicate: approved, object: NP)

STATUS:
  "[NP] is [approved|rejected|verified|blocked|pending|flagged|cleared|on hold]"
  → (subject: NP, predicate: status, object: status_value)

COMPLIANCE_EVENT:
  "[NP] [passed|failed|completed|cleared] [KYC|KYB|AML|audit|compliance check|review]"
  → (subject: NP, predicate: passed|failed, object: compliance_type)

AMOUNT:
  "[NP] [is|costs|amounts to|totals] [$amount]"
  → (subject: NP, predicate: amount, object: currency_amount)

THRESHOLD:
  "[NP] [exceeds|is above|is below|is within] [threshold|limit|amount]"
  → (subject: NP, predicate: exceeds|within, object: threshold_value)

RELATIONSHIP:
  "[NP] [is owned by|is controlled by|manages|is a subsidiary of] [NP]"
  → (subject: NP_1, predicate: relationship_type, object: NP_2)

BLOCK:
  "[NP] [is blocked|has been blocked|was blocked] [by NP]?"
  → (subject: NP, predicate: blocked, object: blocker|null)

CORRECTION:
  "No[,] [the|it] [NP]? [predicate] [NP]"
  → extract corrected triple, set correction_of: prior_fact_id
```

If the message matches a pattern, extract the triple. If it does not match any
pattern, proceed to Step 4.

**Step 4 — spaCy dep-parse fallback (Python sidecar, ~15-20ms)**

Only runs on messages that did not match the grammar parser. Uses `en_core_web_md`
(40MB, ~90% SVO accuracy on formal English, actively maintained).

Replace the current `en_core_web_sm` + dead `coreferee` with:
- `en_core_web_md` for dep-parse and NER
- Simple rule-based pronoun resolution: if subject is a pronoun (`it`, `they`,
  `this`, `the vendor`), check the last two user messages for a financial entity
  of matching type and substitute. This covers 90%+ of pronoun cases in financial
  conversation without a coreference model.

Do not use `coreferee`. It is unmaintained and will not be fixed. Do not use
`fastcoref` for production — it is a research library with no production support.
The rule-based pronoun resolution is auditable, deterministic, and sufficient for
finance.

**Step 5 — Entity resolution against graph (~2ms, Go)**

For each extracted entity string, fuzzy-match (Levenshtein distance ≤ 2 + type
match) against existing fact labels in the session graph. If matched, the new fact
gets a `depends_on` candidate pointing to that node. If not matched, it is a new
root fact.

This is the primary mechanism for cross-message dependency resolution without an
LLM. If the user said "Vendor 17 passed KYC" two messages ago, and now says "the
vendor is approved for payment", entity resolution links "the vendor" to the
existing `vendor-17` node.

**Step 6 — Graph validation (~2ms, Go)**

Same TMS consistency precheck as the controlled LLM path. Contradiction → queue.
Accepted → assert.

**Tradeoff:** The grammar parser covers the finance predicate space but will miss
assertions phrased in unusual ways. The dep-parse fallback handles these but with
lower accuracy.
**Solution:** Track grammar parser hit rate vs fallback rate in production metrics.
If fallback rate exceeds 20%, add new grammar patterns for the missed cases. In
finance, the long tail of unusual phrasing is short — a few months of production
data will close it.

---

### Uncontrolled LLM path (fallback, target: <60ms)

**Why production-ready:** Same spaCy sidecar as the user message path — one shared
process, not two. The sentence classifier below is rule-based. The only accuracy
risk is the dep-parse quality on LLM output, which is high because LLM output is
grammatically well-formed English.

**Step 1 — Sentence classifier (Go, ~1ms)**

Filter sentences from LLM output before sending to spaCy. Drop:
- Sentences starting with "I'll", "Let me", "I can", "Sure", "Of course" (meta-commentary)
- Questions directed back to the user
- Markdown headers and bullet point labels without predicates

Keep declarative sentences with financial content.

**Step 2 — spaCy dep-parse on selected sentences (~20-40ms, Python sidecar)**

Same `en_core_web_md` process as user message fallback. LLM output is grammatically
cleaner than user messages so accuracy is slightly higher (~92%).

**Step 3 — Discourse connective dependency inference (~2ms, Python, rule-based)**

Keep the existing connectives logic from the current sidecar. Sentences opening with
`therefore`, `thus`, `consequently`, `as a result`, `based on`, `given that`,
`since` → dependency edge to the prior sentence's facts.

This replaces the entire O(n²) Stage 3B LLM pairwise checking in the current
pipeline. It is less semantically precise but it is deterministic, fast, and does
not compound LLM errors.

**Step 4 — Entity resolution + graph validation (~4ms, Go)**

Same as user message path.

**Tradeoff:** Discourse connective inference will miss semantic dependencies that
have no lexical signal ("the transaction is cleared" depending on "the KYC was
approved" with no connective between them). These become isolated root facts
instead of derived facts.
**Solution:** This is the safe failure mode. An isolated root fact that should have
been derived does not corrupt the graph — it just means the execute-check does not
track that dependency. When the LLM has system prompt access, the controlled path
resolves this correctly. The uncontrolled path is explicitly a lower-fidelity
fallback.

---

## Finance Predicate List

The exhaustive list of predicates for the finance domain. Any predicate not in this
list is rejected at the schema validation step.

```
approved, rejected, authorized, denied, flagged, cleared, blocked, held,
passed, failed, completed, pending, expired, active, inactive,
amount, total, balance, threshold, limit,
owned-by, controlled-by, manages, subsidiary-of, affiliated-with,
transferred, received, disbursed, refunded, reversed,
verified, unverified, under-review, escalated,
exceeds, within, below, above
```

Extend this list when real production data reveals missing predicates. Do not
attempt to handle arbitrary predicates — that is the path back to the general
extraction problem.

---

## What is Removed from the Current Pipeline

| Current component | Status | Reason |
|---|---|---|
| `en_core_web_sm` | **Replace with `en_core_web_md`** | sm is too inaccurate for production |
| `coreferee` | **Remove entirely** | Unmaintained since 2021, no production support |
| 5-stage LLM pipeline (Tier 3) | **Remove as primary** | O(n²) LLM calls, not scalable |
| Stage 3B pairwise dependency LLM | **Remove entirely** | Replaced by discourse connectives |
| Stage 4 LLM coverage verification | **Remove** | Relies on the LLM checking itself |
| GLiNER | **Keep as optional** | Useful for vendor/entity NER in finance, off by default |
| Circuit breaker on sidecar | **Keep** | Correct production pattern |
| Stage 5 consistency precheck | **Keep** | Deterministic, correct, fast |
| Graph validation gate | **Keep and extend** | The core accuracy mechanism |

---

## Latency Summary

| Path | Target | Breakdown |
|---|---|---|
| Controlled LLM | <10ms | Parser 1ms + validation 2ms + graph 2ms |
| User message (grammar match) | <10ms | Classifier 1ms + entity extract 1ms + grammar 2ms + graph 2ms |
| User message (dep-parse fallback) | <40ms | Above + sidecar 20ms + entity resolution 2ms |
| Uncontrolled LLM | <60ms | Classifier 1ms + sidecar 40ms + connectives 2ms + graph 4ms |

All paths share one Python sidecar process. Models are pre-loaded at startup.
No cold start on first request.

---

## Accuracy Expectation (Finance Domain)

| Path | Expected accuracy | Primary failure mode |
|---|---|---|
| Controlled LLM | ~97% | LLM omits fact from FACTS block |
| User message grammar | ~95%+ on covered patterns | Unusual phrasing not in grammar |
| User message dep-parse | ~88-91% | Complex structure, long sentences |
| Uncontrolled LLM | ~88-92% | Missing semantic dependencies |

The finance domain raises accuracy across all paths because the predicate and entity
space is bounded. A wrong predicate is rejected at schema validation. A wrong entity
is caught at graph validation if it contradicts existing facts. The compounding error
problem is significantly reduced compared to open-domain extraction.
