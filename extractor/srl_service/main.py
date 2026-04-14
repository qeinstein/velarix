"""
SRL Extraction Microservice — Tier 1 classical NLP pipeline.

Exposes POST /extract accepting raw text and returning structured extraction
results (facts, dependency edges, conflict pairs) with zero LLM API calls.

Stages:
  1. Clause boundary detection and simplification (spaCy dep parse)
  2. Coreference resolution (rule-based pronoun resolver)
  3. Named Entity Recognition (spaCy NER)
  4. Semantic Role Labeling (AllenNLP SRL)
  5. Discourse relation classification and dependency inference
"""

from __future__ import annotations

import hashlib
import json
import logging
import os
import re
from pathlib import Path
from typing import Any

import requests
import spacy
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("srl_service")

# ---------------------------------------------------------------------------
# App bootstrap
# ---------------------------------------------------------------------------
app = FastAPI(title="Velarix SRL Extraction Service", version="0.1.0")

# Load spaCy model (sm for broad compatibility; trf can be swapped in)
NLP = spacy.load("en_core_web_sm")

# Load AllenNLP SRL predictor lazily (heavy import)
_SRL_PREDICTOR = None


def _get_srl_predictor():
    """Lazy-load the AllenNLP SRL predictor to avoid import cost on module load."""
    global _SRL_PREDICTOR
    if _SRL_PREDICTOR is not None:
        return _SRL_PREDICTOR
    try:
        from allennlp.predictors.predictor import Predictor

        _SRL_PREDICTOR = Predictor.from_path(
            "https://storage.googleapis.com/allennlp-public-models/"
            "structured-prediction-srl-bert.2020.12.15.tar.gz"
        )
        logger.info("AllenNLP SRL predictor loaded")
    except Exception:
        logger.warning("AllenNLP SRL predictor unavailable; falling back to dep-parse SRL")
        _SRL_PREDICTOR = None
    return _SRL_PREDICTOR


# Load connective lexicon
_CONNECTIVES_PATH = Path(__file__).parent / "connectives.json"
with open(_CONNECTIVES_PATH) as _f:
    CONNECTIVES: dict[str, list[str]] = json.load(_f)

# Flatten for quick lookup: word/phrase -> relation type
_CONNECTIVE_MAP: dict[str, str] = {}
for _rel, _words in CONNECTIVES.items():
    for _w in _words:
        _CONNECTIVE_MAP[_w.lower()] = _rel

# Internal Go server URL for TMS validation
VELARIX_INTERNAL_URL = os.getenv("VELARIX_INTERNAL_URL", "http://localhost:8080")

# NER types we care about
NER_TYPES = {"PERSON", "ORG", "GPE", "DATE", "MONEY", "PERCENT", "PRODUCT", "EVENT", "LAW"}


# ---------------------------------------------------------------------------
# Request / Response models
# ---------------------------------------------------------------------------
class ExtractRequest(BaseModel):
    text: str
    session_context: str = ""
    velarix_internal_url: str = ""


class SimplifiedSentence(BaseModel):
    sentence: str
    source: str
    head: str = ""
    relation: str = ""
    original_index: int = 0


class CoreferenceEntry(BaseModel):
    pronoun_span: str
    antecedent: str
    confidence: float
    resolved: bool


class EntityInfo(BaseModel):
    text: str
    label: str
    start_char: int
    end_char: int


class FactModifiers(BaseModel):
    temporal: str = ""
    location: str = ""
    causal: str = ""
    manner: str = ""


class ExtractedFactResult(BaseModel):
    id: str
    subject: str
    predicate: str
    object: str
    claim: str
    confidence: float
    assertion_kind: str = "empirical"
    modifiers: FactModifiers = FactModifiers()
    source_sentence_index: int = 0
    srl_confidence: float = 0.0
    entity_types: dict[str, str] = {}
    depends_on: list[str] = []
    is_root: bool = True
    polarity: str = "positive"


class ConflictPair(BaseModel):
    fact_a_id: str
    fact_b_id: str
    reason: str = "ambiguous_parse"


class ExtractionStats(BaseModel):
    simplified_sentences: int = 0
    coreferences_resolved: int = 0
    entities_found: int = 0
    facts_extracted: int = 0
    edges_proposed: int = 0
    edges_accepted: int = 0
    edges_rejected: int = 0
    ambiguous_pairs: int = 0
    fallback_sentences: int = 0


class ExtractResponse(BaseModel):
    facts: list[ExtractedFactResult]
    conflict_pairs: list[ConflictPair] = []
    coreference_map: list[CoreferenceEntry] = []
    stats: ExtractionStats = ExtractionStats()


# ---------------------------------------------------------------------------
# Stage 1 — Clause Boundary Detection and Simplification
# ---------------------------------------------------------------------------
_CLAUSE_DEPS = {"relcl", "advcl", "ccomp", "xcomp"}

_DEP_TO_RELATION = {
    "relcl": "relative",
    "advcl": "temporal",  # default; refined by connective detection
    "ccomp": "complement",
    "xcomp": "complement",
}


def _find_subject(token):
    """Walk up the tree to find the nearest nominal subject."""
    for child in token.head.children:
        if child.dep_ in ("nsubj", "nsubjpass"):
            return child.subtree
    # Fallback: use the head noun itself.
    return [token.head]


def stage1_simplify(text: str) -> list[SimplifiedSentence]:
    """Split complex sentences into simple standalone sentences."""
    doc = NLP(text)
    results: list[SimplifiedSentence] = []
    for sent_idx, sent in enumerate(doc.sents):
        main_tokens = list(sent)
        clause_tokens_to_remove: set[int] = set()
        subordinate_clauses: list[dict] = []

        for token in sent:
            if token.dep_ in _CLAUSE_DEPS:
                clause_span = list(token.subtree)
                clause_indices = {t.i for t in clause_span}
                clause_tokens_to_remove.update(clause_indices)

                # Resolve subject for the clause
                subject_tokens = list(_find_subject(token))
                subject_text = " ".join(t.text for t in subject_tokens)

                clause_text = " ".join(t.text for t in clause_span)
                # Prepend subject if the clause doesn't start with it
                clause_lower = clause_text.lower()
                if not clause_lower.startswith(subject_text.lower()):
                    clause_text = subject_text + " " + clause_text

                # Clean up relative pronouns at the start
                clause_text = re.sub(
                    r"^(which|that|who|whom|whose)\s+",
                    "",
                    clause_text,
                    flags=re.IGNORECASE,
                )
                # Capitalize and ensure period
                clause_text = clause_text.strip()
                if clause_text:
                    clause_text = clause_text[0].upper() + clause_text[1:]
                    if not clause_text.endswith("."):
                        clause_text += "."

                relation = _DEP_TO_RELATION.get(token.dep_, "other")
                # Refine advcl relation by looking at the connective
                if token.dep_ == "advcl":
                    for child in token.children:
                        if child.dep_ == "mark":
                            conn_type = _CONNECTIVE_MAP.get(child.text.lower())
                            if conn_type:
                                relation = conn_type
                            break

                subordinate_clauses.append(
                    {
                        "text": clause_text,
                        "source": token.dep_,
                        "head": subject_text,
                        "relation": relation,
                    }
                )

        # Build main clause (remove subordinate tokens)
        main_words = []
        for t in main_tokens:
            if t.i not in clause_tokens_to_remove:
                main_words.append(t.text_with_ws)
        main_text = "".join(main_words).strip()
        # Clean up leftover commas / whitespace
        main_text = re.sub(r"\s*,\s*,", ",", main_text)
        main_text = re.sub(r"\s+", " ", main_text).strip()
        if main_text and not main_text.endswith("."):
            main_text += "."

        if main_text and len(main_text) > 2:
            results.append(
                SimplifiedSentence(
                    sentence=main_text,
                    source="main",
                    original_index=sent_idx,
                )
            )

        for sc in subordinate_clauses:
            if sc["text"] and len(sc["text"]) > 2:
                results.append(
                    SimplifiedSentence(
                        sentence=sc["text"],
                        source=sc["source"],
                        head=sc["head"],
                        relation=sc["relation"],
                        original_index=sent_idx,
                    )
                )

    # If nothing was split, return original sentences
    if not results:
        doc2 = NLP(text)
        for idx, sent in enumerate(doc2.sents):
            s = sent.text.strip()
            if s:
                results.append(SimplifiedSentence(sentence=s, source="main", original_index=idx))

    return results


# ---------------------------------------------------------------------------
# Stage 2 — Coreference Resolution (rule-based)
# ---------------------------------------------------------------------------
_PRONOUNS = {"he", "she", "it", "they", "him", "her", "them", "his", "its", "their"}


def stage2_coreference(
    sentences: list[SimplifiedSentence],
) -> tuple[list[SimplifiedSentence], list[CoreferenceEntry]]:
    """Resolve pronouns across sentences using a simple antecedent heuristic."""
    full_text = " ".join(s.sentence for s in sentences)
    doc = NLP(full_text)

    # Build entity mentions in order
    entity_mentions: list[tuple[str, int]] = []
    for ent in doc.ents:
        if ent.label_ in {"PERSON", "ORG", "GPE", "PRODUCT", "EVENT"}:
            entity_mentions.append((ent.text, ent.start_char))

    # Also collect noun chunk heads as fallback antecedents
    noun_mentions: list[tuple[str, int]] = []
    for chunk in doc.noun_chunks:
        if chunk.root.pos_ in ("NOUN", "PROPN"):
            noun_mentions.append((chunk.text, chunk.start_char))

    all_antecedents = entity_mentions + noun_mentions

    coref_map: list[CoreferenceEntry] = []
    replacements: dict[str, str] = {}  # sentence_idx -> rewritten

    for sent in sentences:
        sent_doc = NLP(sent.sentence)
        new_tokens = list(sent.sentence)
        changed = False

        for token in sent_doc:
            if token.text.lower() in _PRONOUNS:
                # Find the most recent antecedent that appeared before this sentence
                best_ant = None
                best_conf = 0.0

                for ant_text, _ in reversed(all_antecedents):
                    # Simple heuristic: entity mentions get higher confidence
                    is_entity = any(ant_text == em[0] for em in entity_mentions)
                    conf = 0.85 if is_entity else 0.65

                    # Check gender/number compatibility loosely
                    if token.text.lower() in ("he", "him", "his"):
                        # Prefer PERSON entities
                        if is_entity:
                            conf = 0.9
                    elif token.text.lower() in ("she", "her"):
                        if is_entity:
                            conf = 0.9
                    elif token.text.lower() in ("it", "its"):
                        if not is_entity or any(
                            ant_text == em[0]
                            for em in entity_mentions
                            if NLP(em[0]).ents
                            and NLP(em[0]).ents[0].label_ in ("ORG", "GPE", "PRODUCT")
                        ):
                            conf = 0.8

                    if conf > best_conf:
                        best_conf = conf
                        best_ant = ant_text

                if best_ant:
                    resolved = best_conf >= 0.75
                    coref_map.append(
                        CoreferenceEntry(
                            pronoun_span=token.text,
                            antecedent=best_ant,
                            confidence=best_conf,
                            resolved=resolved,
                        )
                    )
                    if resolved:
                        # Replace pronoun in sentence text
                        sent.sentence = re.sub(
                            r"\b" + re.escape(token.text) + r"\b",
                            best_ant,
                            sent.sentence,
                            count=1,
                        )
                        changed = True

    return sentences, coref_map


# ---------------------------------------------------------------------------
# Stage 3 — Named Entity Recognition
# ---------------------------------------------------------------------------
def stage3_ner(sentences: list[SimplifiedSentence]) -> dict[int, list[EntityInfo]]:
    """Run NER on each sentence, returning entities keyed by sentence index."""
    results: dict[int, list[EntityInfo]] = {}
    for idx, sent in enumerate(sentences):
        doc = NLP(sent.sentence)
        ents = []
        for ent in doc.ents:
            if ent.label_ in NER_TYPES:
                ents.append(
                    EntityInfo(
                        text=ent.text,
                        label=ent.label_,
                        start_char=ent.start_char,
                        end_char=ent.end_char,
                    )
                )
        results[idx] = ents
    return results


# ---------------------------------------------------------------------------
# Stage 4 — Semantic Role Labeling
# ---------------------------------------------------------------------------
def _slug(text: str) -> str:
    """Generate a slug from text."""
    s = text.lower().strip()
    s = re.sub(r"[^a-z0-9]+", "-", s)
    s = s.strip("-")
    return s[:64] if s else "fact"


def _fact_id(predicate: str, arg0: str, arg1: str) -> str:
    """Derive a deterministic fact ID from predicate + args."""
    raw = f"{predicate}-{arg0}-{arg1}"
    slug = _slug(raw)
    if not slug:
        slug = hashlib.md5(raw.encode()).hexdigest()[:12]
    return slug


def stage4_srl(
    sentences: list[SimplifiedSentence],
    entities_by_sentence: dict[int, list[EntityInfo]],
) -> list[ExtractedFactResult]:
    """Run SRL and convert predicate-argument structures to fact tuples."""
    facts: list[ExtractedFactResult] = []
    seen_ids: set[str] = set()

    predictor = _get_srl_predictor()

    for idx, sent in enumerate(sentences):
        ents = entities_by_sentence.get(idx, [])
        ent_texts = {e.text for e in ents}
        ent_type_map = {e.text: e.label for e in ents}

        if predictor is not None:
            # Use AllenNLP SRL
            try:
                srl_result = predictor.predict(sentence=sent.sentence)
            except Exception:
                logger.warning("SRL prediction failed for: %s", sent.sentence[:80])
                srl_result = {"verbs": []}

            for verb_entry in srl_result.get("verbs", []):
                tags = verb_entry.get("tags", [])
                words = srl_result.get("words", [])
                description = verb_entry.get("description", "")
                verb = verb_entry.get("verb", "")

                # Extract arguments from BIO tags
                args: dict[str, str] = {}
                current_arg = None
                current_tokens: list[str] = []

                for word, tag in zip(words, tags):
                    if tag.startswith("B-"):
                        if current_arg and current_tokens:
                            args[current_arg] = " ".join(current_tokens)
                        current_arg = tag[2:]
                        current_tokens = [word]
                    elif tag.startswith("I-") and current_arg:
                        current_tokens.append(word)
                    else:
                        if current_arg and current_tokens:
                            args[current_arg] = " ".join(current_tokens)
                        current_arg = None
                        current_tokens = []

                if current_arg and current_tokens:
                    args[current_arg] = " ".join(current_tokens)

                subject = args.get("ARG0", "").strip()
                obj = args.get("ARG1", "").strip()

                if not subject and not obj:
                    continue

                modifiers = FactModifiers(
                    temporal=args.get("ARGM-TMP", "").strip(),
                    location=args.get("ARGM-LOC", "").strip(),
                    causal=args.get("ARGM-CAU", "").strip(),
                    manner=args.get("ARGM-MNR", "").strip(),
                )

                # Compute SRL confidence (AllenNLP doesn't expose per-prediction scores natively)
                srl_conf = 0.8  # baseline for a successful parse

                fid = _fact_id(verb, subject, obj)
                counter = 2
                while fid in seen_ids:
                    fid = f"{_fact_id(verb, subject, obj)}-{counter}"
                    counter += 1
                seen_ids.add(fid)

                # Entity type enrichment
                entity_types = {}
                if subject in ent_type_map:
                    entity_types["subject"] = ent_type_map[subject]
                if obj in ent_type_map:
                    entity_types["object"] = ent_type_map[obj]

                claim = description if description else sent.sentence

                facts.append(
                    ExtractedFactResult(
                        id=fid,
                        subject=subject,
                        predicate=verb,
                        object=obj,
                        claim=claim,
                        confidence=srl_conf,
                        assertion_kind="empirical",
                        modifiers=modifiers,
                        source_sentence_index=idx,
                        srl_confidence=srl_conf,
                        entity_types=entity_types,
                    )
                )
        else:
            # Fallback: dep-parse based SRL
            doc = NLP(sent.sentence)
            for token in doc:
                if token.pos_ != "VERB":
                    continue

                subject = ""
                obj = ""
                temporal = ""
                location = ""

                for child in token.children:
                    if child.dep_ in ("nsubj", "nsubjpass"):
                        subject = " ".join(t.text for t in child.subtree)
                    elif child.dep_ in ("dobj", "attr", "oprd"):
                        obj = " ".join(t.text for t in child.subtree)
                    elif child.dep_ in ("prep",) and any(
                        gc.dep_ == "pobj" for gc in child.children
                    ):
                        prep_phrase = " ".join(t.text for t in child.subtree)
                        if child.text.lower() in ("in", "at", "on", "near"):
                            location = prep_phrase
                        elif child.text.lower() in ("after", "before", "during", "since"):
                            temporal = prep_phrase

                if not subject and not obj:
                    continue

                srl_conf = 0.7  # lower confidence for dep-parse fallback

                fid = _fact_id(token.lemma_, subject, obj)
                counter = 2
                while fid in seen_ids:
                    fid = f"{_fact_id(token.lemma_, subject, obj)}-{counter}"
                    counter += 1
                seen_ids.add(fid)

                entity_types = {}
                if subject in ent_type_map:
                    entity_types["subject"] = ent_type_map[subject]
                if obj in ent_type_map:
                    entity_types["object"] = ent_type_map[obj]

                facts.append(
                    ExtractedFactResult(
                        id=fid,
                        subject=subject,
                        predicate=token.lemma_,
                        object=obj,
                        claim=sent.sentence,
                        confidence=srl_conf,
                        assertion_kind="empirical",
                        modifiers=FactModifiers(temporal=temporal, location=location),
                        source_sentence_index=idx,
                        srl_confidence=srl_conf,
                        entity_types=entity_types,
                    )
                )

    return facts


# ---------------------------------------------------------------------------
# Stage 5 — Discourse Relation Classification & Dependency Inference
# ---------------------------------------------------------------------------

# 5A — Explicit connective detection
def stage5a_connective_edges(
    sentences: list[SimplifiedSentence],
    facts: list[ExtractedFactResult],
) -> list[dict]:
    """Detect discourse connectives between clauses and propose dependency edges."""
    edges: list[dict] = []

    # Map sentence indices to facts
    facts_by_sentence: dict[int, list[ExtractedFactResult]] = {}
    for f in facts:
        facts_by_sentence.setdefault(f.source_sentence_index, []).append(f)

    for sent in sentences:
        lower = sent.sentence.lower()
        for phrase, rel_type in _CONNECTIVE_MAP.items():
            if phrase in lower:
                # Find facts in this sentence and the previous main clause
                current_facts = facts_by_sentence.get(sent.original_index, [])
                # Look for the main clause that this subordinate is attached to
                for other_sent in sentences:
                    if other_sent.source == "main" and other_sent.original_index == sent.original_index:
                        main_facts = facts_by_sentence.get(
                            sentences.index(other_sent), []
                        )
                        for cf in current_facts:
                            for mf in main_facts:
                                if cf.id != mf.id:
                                    edges.append(
                                        {
                                            "parent_id": mf.id,
                                            "child_id": cf.id,
                                            "type": rel_type,
                                            "confidence": 0.9,
                                            "source": "connective",
                                        }
                                    )
                break  # Only need the first connective match per sentence

    return edges


# 5B — Entity overlap dependency inference
def stage5b_entity_overlap_edges(facts: list[ExtractedFactResult]) -> list[dict]:
    """Create candidate edges where subject/object of B matches an entity in A."""
    edges: list[dict] = []
    for i, fact_a in enumerate(facts):
        a_entities = {fact_a.subject.lower(), fact_a.object.lower()} - {""}
        for j, fact_b in enumerate(facts):
            if i == j:
                continue
            b_subject = fact_b.subject.lower()
            b_object = fact_b.object.lower()
            # Check if B's subject or object appears in A's entities
            overlap = False
            if b_subject and b_subject in a_entities:
                overlap = True
            if b_object and b_object in a_entities:
                overlap = True
            if overlap and fact_a.id != fact_b.id:
                edges.append(
                    {
                        "parent_id": fact_a.id,
                        "child_id": fact_b.id,
                        "type": "entity_overlap",
                        "confidence": 0.7,
                        "source": "entity_overlap",
                    }
                )
    return edges


# 5C — TMS constraint validation
def stage5c_tms_validate(
    edges: list[dict],
    facts: list[ExtractedFactResult],
    internal_url: str,
) -> tuple[list[dict], int, int]:
    """Validate candidate edges against a temporary TMS engine instance."""
    accepted: list[dict] = []
    rejected_count = 0

    # Build minimal fact representation for validation
    fact_dicts = []
    for f in facts:
        fact_dicts.append(
            {
                "id": f.id,
                "claim": f.claim,
                "subject": f.subject,
                "predicate": f.predicate,
                "object": f.object,
                "confidence": f.confidence,
                "assertion_kind": f.assertion_kind,
                "depends_on": list(f.depends_on),
                "is_root": f.is_root,
            }
        )

    # Sort edges by confidence (highest first)
    sorted_edges = sorted(edges, key=lambda e: e["confidence"], reverse=True)

    # Deduplicate edges
    seen_pairs: set[tuple[str, str]] = set()
    unique_edges = []
    for edge in sorted_edges:
        pair = (edge["parent_id"], edge["child_id"])
        if pair not in seen_pairs and edge["parent_id"] != edge["child_id"]:
            seen_pairs.add(pair)
            unique_edges.append(edge)

    for edge in unique_edges:
        # Try to validate via the Go server's internal endpoint
        try:
            resp = requests.post(
                f"{internal_url}/internal/validate-dependency",
                json={
                    "parent_id": edge["parent_id"],
                    "child_id": edge["child_id"],
                    "facts": fact_dicts,
                },
                timeout=2,
            )
            if resp.status_code == 200:
                result = resp.json()
                if result.get("accepted", False):
                    accepted.append(edge)
                else:
                    rejected_count += 1
            else:
                # If the validation endpoint is not available, use local cycle check
                if _local_acyclic_check(edge, accepted, facts):
                    accepted.append(edge)
                else:
                    rejected_count += 1
        except (requests.RequestException, Exception):
            # Fallback to local acyclic validation
            if _local_acyclic_check(edge, accepted, facts):
                accepted.append(edge)
            else:
                rejected_count += 1

    return accepted, len(unique_edges), rejected_count


def _local_acyclic_check(
    new_edge: dict,
    accepted: list[dict],
    facts: list[ExtractedFactResult],
) -> bool:
    """Simple local cycle detection when TMS endpoint is unavailable."""
    # Build adjacency list from accepted edges
    children_of: dict[str, set[str]] = {}
    for e in accepted:
        children_of.setdefault(e["parent_id"], set()).add(e["child_id"])

    # Add proposed edge
    children_of.setdefault(new_edge["parent_id"], set()).add(new_edge["child_id"])

    # DFS cycle detection
    visited: set[str] = set()
    in_stack: set[str] = set()

    fact_ids = {f.id for f in facts}

    def has_cycle(node: str) -> bool:
        if node in in_stack:
            return True
        if node in visited:
            return False
        visited.add(node)
        in_stack.add(node)
        for child in children_of.get(node, set()):
            if has_cycle(child):
                return True
        in_stack.discard(node)
        return False

    for fid in fact_ids:
        visited.clear()
        in_stack.clear()
        if has_cycle(fid):
            return False
    return True


# 5D — Ambiguity handling
def stage5d_ambiguity(
    facts: list[ExtractedFactResult],
    srl_threshold: float = 0.8,
) -> tuple[list[ExtractedFactResult], list[ConflictPair]]:
    """Handle ambiguous parses by creating hypothetical fact pairs."""
    conflict_pairs: list[ConflictPair] = []

    # For facts with very low SRL confidence, we might have multiple interpretations
    # In practice, AllenNLP returns multiple verb frames per sentence — if the top
    # frame's confidence is low, we treat co-occurring frames as hypothetical pairs.
    low_conf_groups: dict[int, list[ExtractedFactResult]] = {}
    for f in facts:
        if f.srl_confidence < srl_threshold:
            low_conf_groups.setdefault(f.source_sentence_index, []).append(f)

    for sent_idx, group in low_conf_groups.items():
        if len(group) >= 2:
            # Mark both as hypothetical
            for f in group:
                f.assertion_kind = "hypothetical"
            # Record the conflict pair (first two)
            conflict_pairs.append(
                ConflictPair(
                    fact_a_id=group[0].id,
                    fact_b_id=group[1].id,
                    reason="ambiguous_parse",
                )
            )

    return facts, conflict_pairs


# ---------------------------------------------------------------------------
# Confidence scoring
# ---------------------------------------------------------------------------
def compute_final_confidence(
    fact: ExtractedFactResult,
    coref_confidence: float,
    entities_by_sentence: dict[int, list[EntityInfo]],
) -> float:
    """Compute weighted confidence: (srl×0.5) + (coref×0.3) + (entity_match×0.2)."""
    ents = entities_by_sentence.get(fact.source_sentence_index, [])
    ent_texts = {e.text.lower() for e in ents}

    subject_is_entity = fact.subject.lower() in ent_texts if fact.subject else False
    object_is_entity = fact.object.lower() in ent_texts if fact.object else False

    if subject_is_entity and object_is_entity:
        entity_match = 1.0
    elif subject_is_entity or object_is_entity:
        entity_match = 0.7
    else:
        entity_match = 0.4

    final = (fact.srl_confidence * 0.5) + (coref_confidence * 0.3) + (entity_match * 0.2)
    return round(final, 4)


# ---------------------------------------------------------------------------
# POST /extract
# ---------------------------------------------------------------------------
@app.post("/extract", response_model=ExtractResponse)
def extract(req: ExtractRequest) -> ExtractResponse:
    """Run the full 5-stage SRL extraction pipeline."""
    text = req.text.strip()
    if not text:
        raise HTTPException(status_code=400, detail="text is required")

    internal_url = req.velarix_internal_url or VELARIX_INTERNAL_URL
    stats = ExtractionStats()

    # Stage 1 — Clause simplification
    simplified = stage1_simplify(text)
    stats.simplified_sentences = len(simplified)

    # Stage 2 — Coreference resolution
    resolved_sentences, coref_map = stage2_coreference(simplified)
    stats.coreferences_resolved = sum(1 for c in coref_map if c.resolved)

    # Compute average coref confidence for scoring
    coref_confs = [c.confidence for c in coref_map if c.resolved]
    avg_coref_conf = sum(coref_confs) / len(coref_confs) if coref_confs else 0.85

    # Stage 3 — NER
    entities_by_sentence = stage3_ner(resolved_sentences)
    stats.entities_found = sum(len(v) for v in entities_by_sentence.values())

    # Stage 4 — SRL
    facts = stage4_srl(resolved_sentences, entities_by_sentence)
    stats.facts_extracted = len(facts)

    # Stage 5A — Connective detection
    connective_edges = stage5a_connective_edges(resolved_sentences, facts)

    # Stage 5B — Entity overlap
    overlap_edges = stage5b_entity_overlap_edges(facts)

    # Combine candidate edges
    all_candidate_edges = connective_edges + overlap_edges

    # Stage 5C — TMS validation
    accepted_edges, proposed_count, rejected_count = stage5c_tms_validate(
        all_candidate_edges, facts, internal_url
    )
    stats.edges_proposed = proposed_count
    stats.edges_accepted = len(accepted_edges)
    stats.edges_rejected = rejected_count

    # Apply accepted edges to facts
    parent_map: dict[str, list[str]] = {}
    for edge in accepted_edges:
        parent_map.setdefault(edge["child_id"], []).append(edge["parent_id"])

    for fact in facts:
        parents = parent_map.get(fact.id, [])
        if parents:
            fact.depends_on = parents
            fact.is_root = False

    # Stage 5D — Ambiguity handling
    facts, conflict_pairs = stage5d_ambiguity(facts)
    stats.ambiguous_pairs = len(conflict_pairs)

    # Final confidence scoring
    for fact in facts:
        final_conf = compute_final_confidence(fact, avg_coref_conf, entities_by_sentence)
        fact.confidence = final_conf

        # Low confidence → uncertain
        if final_conf < 0.5:
            fact.assertion_kind = "uncertain"

        # Low SRL confidence → uncertain
        if fact.srl_confidence < 0.6:
            fact.assertion_kind = "uncertain"

    return ExtractResponse(
        facts=facts,
        conflict_pairs=conflict_pairs,
        coreference_map=coref_map,
        stats=stats,
    )


# ---------------------------------------------------------------------------
# Health check
# ---------------------------------------------------------------------------
@app.get("/health")
def health():
    return {"status": "ok", "service": "srl_extraction"}


if __name__ == "__main__":
    import uvicorn

    port = int(os.getenv("SRL_SERVICE_PORT", "8090"))
    uvicorn.run(app, host="0.0.0.0", port=port)
