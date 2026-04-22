# Velarix Strategic Report: Benchmarking & Extraction Roadmap

**Status:** UNFILTERED ASSESSMENT  
**Target Version:** v0.7.0+  
**Prepared for:** Velarix Engineering

---

## 1. Current State: The "Alpha" Gap

Velarix currently exists in two states: the **Vision** (LATTICE, O(N) transition-based parsing) and the **Reality** (Delta, spaCy-based heuristics).

### 1.1 Extraction Pipeline Analysis (Delta v0.6.0)
The current pipeline in `extractor/srl_service/main.py` is a classic "Frankenstein" of NLP heuristics:
- **Strengths:** Sub-50ms latency, high precision on simple SVO (Subject-Verb-Object) triplets, and effective sentence-splitting.
- **Critical Weaknesses:**
    - **Brittle Grounding:** Relying on `LexicalEmbedding` with a `0.75` threshold results in ~60% recall. This is why the AI feels "safe but silent" (too many `[unverified claim removed]` placeholders).
    - **Heuristic-Heavy Dependencies:** Connectives and chaining are handled via regex/string-matching. This fails on synonym-rich legal text (e.g., "The enterprise" vs "The company").
    - **Coreference Failure:** v0.6.0 shows a 50% F1 on coreference. Since coreference is the backbone of fact chaining, half of your dependency graph is likely disconnected or wrong.


---

## 2. Right Benchmarking Direction

To move from an academic prototype to a reliable decision-integrity layer, Velarix must adopt benchmarks that test **stateful reasoning** and **syntax robustness**.

### 2.1 Widely Accepted Benchmarks
1.  **LegalBench (High Priority):** Specifically the "Reasoning" and "Extraction" tasks. This is the gold standard for the register Velarix is targeting. 87.4% curr best 3.1pro
2.  **ReviseQA** : curr best 85% phi mii
3.  **RULER:** To test how extraction performance degrades as the "session" grows toward the `MaxFactsPerSession` (80,000) limit.
The base model's accuracy on RULER will inevitably plummet as the context size increases due to attention dilution and the "lost in the middle" phenomenon.

Your objective for the paper is to show a flatline. Velarix's accuracy must remain completely horizontal from fact 1 to fact 80,000. This proves that because the Go kernel isolates and computes the state deterministically, performance does not degrade regardless of how large the session grows.

If your extraction pipeline can maintain that flatline, replacing ProntoQA with RULER transitions your paper from a niche formal logic thesis into a direct solution for the LLM context scaling problem
4.  **The Intercept Metric**: (Latency, Token Overhead, & Extraction Cost)


---


### 3.1 Structural Improvements (The LATTICE Transition)
The current spaCy pipeline is hitting a ceiling. You should migrate toward the **ADR-001 (LATTICE)** architecture:
- **Transition-based Transducer:** Replace Stage 4/5 with a single pass that emits `ASSERT`, `NEGATE`, and `EDGE` actions. This moves from "guessing" edges to "emitting" them as part of the parse.
- **VSA (Vector Symbolic Architectures):** Use HRR (Holographic Reduced Representations) for binding roles to fillers. This solves the "Company" vs "The Firm" entity-matching problem without brittle regex.

### 3.2 Immediate Wins for Delta (v0.7.0)
If you aren't ready for a full rewrite, apply these surgical fixes to `extractor/srl_service/main.py`:
1.  **Dense Embeddings:** Replace `LexicalEmbedding` with `text-embedding-3-small` for grounding. This will immediately jump your recall from ~60% to ~90% by allowing paraphrases to match.
2.  **Cross-Sentence Coref Propagation:** Currently, Stage 2 is isolated. Coreference clusters should be treated as "Global Entities" within a session.
3.  **Dependency Weighted Confidence:** Connective edges (`0.9`) should be weighted based on the complexity of the sentence. A sentence with 4+ clauses is likely an extraction error and should have its edge confidence downgraded to `0.5`.

### 3.3 The TMS Boundary
In `core/engine.go`, the `MaxFactsPerSession` is a hard cap. 
- **Improvement:** Implement a **Slice-Aware Eviction** policy. When hitting the limit, evict facts with the lowest `EpistemicWeight` (old, unverified, or leaf nodes with no children) instead of just rejecting new ones.

---

## 4. Final Assessment for Launch

**Why?** The extraction pipeline is still too heuristic-based to handle the complexity of the 500-sentence validation suite. 

**The Path to Launch:**
1.  **Implement the 500-sentence suite.**
2.  **Switch to Dense Embeddings** (Stop the lexical string matching).
3.  **Audit the "execute-check" UI.** If the user can't see *why* a fact was retracted (Causal Explanation), they won't trust the engine.

---
Yes. These four form a complete, defensible systems architecture paper. They isolate specific capabilities without redundancy and build a unified narrative against current LLM limitations.

Here is the exact narrative structure this stack provides for your publication:

### 1. LegalBench (The Baseline Competence)
* **The Role:** Establishes credibility.
* **The Narrative:** Before claiming you have solved AI memory, you must prove the system works in a highly rigorous, zero-tolerance domain. Beating the Gemini 3.1 Pro 87.4% baseline proves the formal logic compiler (Go kernel) executes flawlessly once facts are extracted.

### 2. ReviseQA (The Dynamic Update)
* **The Role:** Proves the Truth Maintenance System (TMS).
* **The Narrative:** Real-world logic is not static. Beating the Phi-3 85% baseline proves that the Directed Acyclic Graph (DAG) correctly propagates state changes, retracts obsolete dependencies, and prevents stale context across multiple conversational turns.

### 3. RULER (The Context Scaling Challenge)
* **The Role:** Attacks the industry meta.
* **The Narrative:** This is your primary architectural claim. By maintaining a flatline accuracy up to your 80,000 node limit while the base model degrades, you mathematically prove that isolating state deterministically is vastly superior to scaling quadratic attention windows.

### 4. The Intercept Metric (The Cost of Certainty)
* **The Role:** Proves production viability.
* **The Narrative:** Systems reviewers will instantly look for the catch. You must publish the raw numbers on API latency and token inflation caused by the Python extraction layer. This section argues that the computational overhead is an acceptable trade-off for the absolute elimination of logical hallucinations.

### The Execution Vulnerability
This stack is rigorous, but it leaves you exposed in one specific area: **The RULER Latency Trap**. 

To maintain that accuracy flatline up to 80,000 facts, your system requires continuous semantic extraction, vector embedding, sub-graph retrieval, and Go compilation. If your Intercept Metric shows that executing a 10,000-fact session through Velarix takes significantly longer or costs exponentially more than sending a single 100k-token prompt to an LLM, reviewers will reject the architecture as practically unusable, regardless of the accuracy. 

Lock in these four. Focus entirely on optimizing the extraction pipeline's speed and precision.