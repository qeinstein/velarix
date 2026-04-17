"""
Delta — Velarix fact extraction pipeline.

Delta converts raw text into atomic (subject, predicate, object) facts with inferred
dependencies, ready for assertion into the Velarix TMS.

v0.5.0 changes (dependency accuracy — 76% → 90%+ F1):
- New: cross-sentence connective pass in stage 5A — sentence-opening connectives
  ("However,", "Subsequently,") now link to the previous sentence's facts, not just
  subordinate clauses. Recovers the ~5 connective misses where the connective was the
  sentence opener rather than a subordinating conjunction.
- New: entity-length filter in stage 5B — single-token common nouns ("company",
  "board", "contract") no longer anchor entity-overlap edges unless NER recognises
  them as a named entity. Eliminates the main source of false-positive edges.
- New: non-human "it" heuristic in rule-based coref fallback — when "it" appears and
  the previous sentence has exactly one non-person noun chunk, resolve "it" to that
  noun. Covers the dominant financial/legal pattern ("the loan was approved. It was
  disbursed…"). Neural coreferee is unaffected; heuristic only fires in rule-based mode.

v0.4.0 changes (stage 5 dependency accuracy overhaul):
- Fix: stage 5A used original_index instead of list-position index when looking up facts,
  silently dropping connective edges for all split sentences.
- Fix: connective matching now uses word-boundary regex instead of plain substring —
  eliminates false positives from short connectives inside longer words.
- Fix: all matching connectives in a sentence are now processed (not just the first).
- Fix: entity overlap strips leading articles before comparison ("the vendor" == "vendor").
- Fix: entity overlap emits each pair once (A→B only, not both A→B and B→A).
- New: entity-type weighted overlap confidence (0.88 named / 0.70 typed / 0.45 common noun).
- New stage 5E: subject→object chaining — edges where A's object is B's subject.
- New stage 5F: coreference-driven edges — resolved coref chains generate dependencies.
- connectives.json expanded: therefore, thus, hence, accordingly, subsequently, etc.

v0.3.0 changes:
- nlp.pipe() batching in stage 4 (one model call per request instead of N serial calls).
- senter pipe disabled at load time (parser already sets sentence boundaries).
- 60-token sentence length cap to protect p99 latency on extreme inputs.

v0.2.0 changes:
- Removed AllenNLP (200MB, lazy cold-start, flat 0.8 confidence). Dep-parse is now primary RE.
- Added coreferee neural coreference (replaces rule-based pronoun heuristic).
- Added GLiNER NER (replaces spaCy NER, better entity coverage and zero-shot labels).
- Batch TMS validation: one HTTP call replaces N serial calls.
- All models pre-loaded at startup — zero cold-start on first request.
- NLP runs once per request on full text; stages 1 and 2 reuse the same Doc.

Stages:
  1. Clause boundary detection and simplification (spaCy dep parse)
  2. Coreference resolution (coreferee neural or rule-based fallback)
  3. Named Entity Recognition (GLiNER or spaCy fallback)
  4. Relation extraction (enhanced dep-parse: SVO, passive, copular, xcomp, conjoined predicates)
  5. Dependency inference:
     5A. Discourse connective edges (typed: causal, temporal, conditional, contrastive)
     5B. Entity overlap edges (named-entity weighted confidence)
     5C. TMS batch validation
     5D. Ambiguity detection
     5E. Subject→object chaining edges
     5F. Coreference-driven edges
"""

from __future__ import annotations

import hashlib
import json
import logging
import os
import re
from pathlib import Path
from typing import Optional

import requests
import spacy
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from spacy.tokens import Doc

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("delta")

# ---------------------------------------------------------------------------
# App bootstrap — all models pre-loaded here, not lazily
# ---------------------------------------------------------------------------
app = FastAPI(title="Velarix Delta Extraction Service", version="0.6.0")

logger.info("Loading spaCy en_core_web_sm...")
NLP = spacy.load("en_core_web_sm")
# Disable pipes we never read from — saves ~3–5ms per request.
# senter is redundant (parser already sets sentence boundaries).
# attribute_ruler MUST stay — it maps tag_ → pos_ (UPOS), which stage4 reads.
_UNUSED_PIPES = [p for p in ("senter",) if p in NLP.pipe_names]
if _UNUSED_PIPES:
    NLP.disable_pipes(*_UNUSED_PIPES)
    logger.info("Disabled unused spaCy pipes: %s", _UNUSED_PIPES)

_MAX_SENT_TOKENS = 60  # sentences longer than this are dep-parse expensive; cap them

# Neural coreference — coreferee. Graceful fallback if not installed.
COREFEREE_AVAILABLE = False
try:
    import coreferee  # noqa: F401 — side-effect: registers the pipe factory
    NLP.add_pipe("coreferee")
    COREFEREE_AVAILABLE = True
    logger.info("coreferee loaded")
except Exception as exc:
    logger.warning("coreferee unavailable (%s) — using rule-based coref fallback", exc)

# GLiNER NER — optional, requires ~600MB RAM headroom.
# Falls back to spaCy NER automatically.
# Set VELARIX_ENABLE_GLINER=1 to opt in; off by default to avoid OOM on small hosts.
GLINER_MODEL = None
GLINER_AVAILABLE = False
if os.environ.get("VELARIX_ENABLE_GLINER") == "1":
    try:
        import psutil  # noqa: E402

        _avail_mb = psutil.virtual_memory().available // (1024 * 1024)
        if _avail_mb < 800:
            logger.warning(
                "GLiNER skipped: only %dMB RAM available (need ≥800MB) — using spaCy NER fallback",
                _avail_mb,
            )
        else:
            from gliner import GLiNER, GLiNERConfig  # noqa: E402
            from huggingface_hub import try_to_load_from_cache  # noqa: E402
            import torch, json as _json  # noqa: E402
            from transformers import AutoTokenizer as _AT  # noqa: E402

            logger.info("Loading GLiNER urchade/gliner_small-v2.1 (%dMB available)...", _avail_mb)

            os.environ.setdefault("HF_HUB_OFFLINE", "1")
            os.environ.setdefault("TRANSFORMERS_OFFLINE", "1")

            _bin = try_to_load_from_cache("urchade/gliner_small-v2.1", "pytorch_model.bin")
            if not _bin:
                raise RuntimeError("GLiNER weights not cached — run: python3 -c \"from gliner import GLiNER; GLiNER.from_pretrained('urchade/gliner_small-v2.1')\"")

            _snap = str(Path(_bin).parent)
            _cfg = GLiNERConfig(**_json.load(open(f"{_snap}/gliner_config.json")))
            _tok = _AT.from_pretrained(_cfg.model_name)
            GLINER_MODEL = GLiNER(_cfg, tokenizer=_tok, encoder_from_pretrained=False)
            import warnings as _w; _w.filterwarnings("ignore")
            _state = torch.load(_bin, map_location="cpu", weights_only=False)
            GLINER_MODEL.model.load_state_dict(_state, strict=False)
            GLINER_MODEL.model.to("cpu")
            GLINER_MODEL.eval()
            GLINER_AVAILABLE = True
            logger.info("GLiNER loaded")
    except Exception as exc:
        logger.warning("GLiNER unavailable (%s) — using spaCy NER fallback", exc)
else:
    logger.info("GLiNER disabled (set VELARIX_ENABLE_GLINER=1 to enable) — using spaCy NER")

# GLiNER zero-shot labels → our NER_TYPES
_GLINER_LABELS = [
    "person", "organization", "location", "country", "city",
    "date", "time", "money", "percentage", "product", "event",
    "law", "regulation",
]
_GLINER_TO_NER: dict[str, str] = {
    "person": "PERSON",
    "organization": "ORG",
    "location": "GPE",
    "country": "GPE",
    "city": "GPE",
    "date": "DATE",
    "time": "DATE",
    "money": "MONEY",
    "percentage": "PERCENT",
    "product": "PRODUCT",
    "event": "EVENT",
    "law": "LAW",
    "regulation": "LAW",
}

# spaCy NER fallback label set
NER_TYPES = {"PERSON", "ORG", "GPE", "DATE", "MONEY", "PERCENT", "PRODUCT", "EVENT", "LAW"}

# Connective lexicon
_CONNECTIVES_PATH = Path(__file__).parent / "connectives.json"
with open(_CONNECTIVES_PATH) as _f:
    CONNECTIVES: dict[str, list[str]] = json.load(_f)

_CONNECTIVE_MAP: dict[str, str] = {}
for _rel, _words in CONNECTIVES.items():
    for _w in _words:
        _CONNECTIVE_MAP[_w.lower()] = _rel

# Pre-compiled word-boundary patterns for each connective (fix 2: no substring false-positives).
_CONNECTIVE_RE: dict[str, re.Pattern] = {
    phrase: re.compile(r"\b" + re.escape(phrase) + r"\b", re.IGNORECASE)
    for phrase in _CONNECTIVE_MAP
}

# Strip leading articles before entity comparison (fix 3: "the vendor" == "vendor").
_DET_RE = re.compile(r"^(the|a|an)\s+", re.IGNORECASE)

# Internal Go server URL for TMS validation
VELARIX_INTERNAL_URL = os.getenv("VELARIX_INTERNAL_URL", "http://localhost:8080")

# Copular verbs — higher-confidence RE pattern
_COPULAR_LEMMAS = {"be", "become", "seem", "appear", "remain", "stay", "get", "turn"}

# Pronouns for rule-based coref fallback
_PRONOUNS = {"he", "she", "it", "they", "him", "her", "them", "his", "its", "their"}


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
    "advcl": "temporal",
    "ccomp": "complement",
    "xcomp": "complement",
}


def _find_subject(token):
    for child in token.head.children:
        if child.dep_ in ("nsubj", "nsubjpass"):
            return child.subtree
    return [token.head]


def stage1_simplify(text: str, doc: Optional[Doc] = None) -> list[SimplifiedSentence]:
    """Split complex sentences into simple standalone sentences.

    Accepts an optional pre-computed spaCy Doc to avoid redundant NLP calls.
    """
    if doc is None:
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

                subject_tokens = list(_find_subject(token))
                subject_text = " ".join(t.text for t in subject_tokens)

                clause_text = " ".join(t.text for t in clause_span)
                if not clause_text.lower().startswith(subject_text.lower()):
                    clause_text = subject_text + " " + clause_text

                clause_text = re.sub(
                    r"^(which|that|who|whom|whose)\s+",
                    "",
                    clause_text,
                    flags=re.IGNORECASE,
                )
                clause_text = clause_text.strip()
                if clause_text:
                    clause_text = clause_text[0].upper() + clause_text[1:]
                    if not clause_text.endswith("."):
                        clause_text += "."

                relation = _DEP_TO_RELATION.get(token.dep_, "other")
                if token.dep_ == "advcl":
                    for child in token.children:
                        if child.dep_ == "mark":
                            conn_type = _CONNECTIVE_MAP.get(child.text.lower())
                            if conn_type:
                                relation = conn_type
                            break

                subordinate_clauses.append({
                    "text": clause_text,
                    "source": token.dep_,
                    "head": subject_text,
                    "relation": relation,
                })

        main_words = []
        for t in main_tokens:
            if t.i not in clause_tokens_to_remove:
                main_words.append(t.text_with_ws)
        main_text = "".join(main_words).strip()
        main_text = re.sub(r"\s*,\s*,", ",", main_text)
        main_text = re.sub(r"\s+", " ", main_text).strip()
        if main_text and not main_text.endswith("."):
            main_text += "."

        if main_text and len(main_text) > 2:
            results.append(SimplifiedSentence(
                sentence=main_text,
                source="main",
                original_index=sent_idx,
            ))

        for sc in subordinate_clauses:
            if sc["text"] and len(sc["text"]) > 2:
                results.append(SimplifiedSentence(
                    sentence=sc["text"],
                    source=sc["source"],
                    head=sc["head"],
                    relation=sc["relation"],
                    original_index=sent_idx,
                ))

    if not results:
        for idx, sent in enumerate(doc.sents):
            s = sent.text.strip()
            if s:
                results.append(SimplifiedSentence(
                    sentence=s, source="main", original_index=idx,
                ))

    return results


# ---------------------------------------------------------------------------
# Stage 2 — Coreference Resolution
# ---------------------------------------------------------------------------

def stage2_coreference(
    sentences: list[SimplifiedSentence],
    full_doc: Doc,
) -> tuple[list[SimplifiedSentence], list[CoreferenceEntry]]:
    """Resolve coreferences using coreferee neural chains when available."""
    coref_map: list[CoreferenceEntry] = []

    if COREFEREE_AVAILABLE:
        replacements: list[tuple[str, str]] = []

        for chain in full_doc._.coref_chains:
            mentions = list(chain)
            if len(mentions) < 2:
                continue

            first_tokens = list(mentions[0])
            antecedent_text = " ".join(full_doc[i].text for i in first_tokens)

            for mention in mentions[1:]:
                m_tokens = list(mention)
                mention_text = " ".join(full_doc[i].text for i in m_tokens)

                if mention_text.lower() in _PRONOUNS:
                    replacements.append((mention_text, antecedent_text))
                    coref_map.append(CoreferenceEntry(
                        pronoun_span=mention_text,
                        antecedent=antecedent_text,
                        confidence=0.88,
                        resolved=True,
                    ))

        for sent in sentences:
            for pronoun, antecedent in replacements:
                sent.sentence = re.sub(
                    r"\b" + re.escape(pronoun) + r"\b",
                    antecedent,
                    sent.sentence,
                    count=1,
                )

        return sentences, coref_map

    # Rule-based fallback
    return _stage2_rulebased(sentences, coref_map)


def _stage2_rulebased(
    sentences: list[SimplifiedSentence],
    coref_map: list[CoreferenceEntry],
) -> tuple[list[SimplifiedSentence], list[CoreferenceEntry]]:
    full_text = " ".join(s.sentence for s in sentences)
    doc = NLP(full_text)

    entity_mentions: list[tuple[str, int]] = []
    for ent in doc.ents:
        if ent.label_ in {"PERSON", "ORG", "GPE", "PRODUCT", "EVENT"}:
            entity_mentions.append((ent.text, ent.start_char))

    noun_mentions: list[tuple[str, int]] = []
    for chunk in doc.noun_chunks:
        if chunk.root.pos_ in ("NOUN", "PROPN"):
            noun_mentions.append((chunk.text, chunk.start_char))

    all_antecedents = entity_mentions + noun_mentions

    # Non-human pronoun heuristics: when "it" or "they/them/their" appears and the
    # immediately preceding sentence has exactly one non-person noun chunk, resolve
    # the pronoun to that noun. Covers financial/legal patterns like:
    #   "The loan was approved. It was disbursed within three days."
    #   "The board reviewed the proposal. They rejected it unanimously."
    sent_texts = [s.sentence for s in sentences]

    def _is_person_chunk(chunk, doc) -> bool:
        return any(
            ent.label_ == "PERSON"
            for ent in doc.ents
            if ent.start <= chunk.start < ent.end
        )

    for sent_idx, sent in enumerate(sentences):
        if sent_idx == 0:
            continue
        lower_words = set(sent.sentence.lower().split())
        has_it   = "it"   in lower_words
        has_they = bool(lower_words & {"they", "them", "their"})
        if not has_it and not has_they:
            continue

        prev_sent_doc = NLP(sent_texts[sent_idx - 1])
        non_person_chunks = [
            ch for ch in prev_sent_doc.noun_chunks
            if ch.root.pos_ in ("NOUN", "PROPN")
            and not _is_person_chunk(ch, prev_sent_doc)
        ]
        if len(non_person_chunks) != 1:
            continue

        antecedent = non_person_chunks[0].text

        if has_it:
            coref_map.append(CoreferenceEntry(
                pronoun_span="it",
                antecedent=antecedent,
                confidence=0.72,
                resolved=True,
            ))
            sent.sentence = re.sub(
                r"\bIt\b", antecedent.capitalize(),
                re.sub(r"\bit\b", antecedent, sent.sentence),
                count=1,
            )

        if has_they:
            for pronoun, replacement in [
                ("They", antecedent.capitalize()),
                ("they", antecedent),
                ("Them", antecedent.capitalize()),
                ("them", antecedent),
                ("Their", antecedent.capitalize() + "'s"),
                ("their", antecedent.lower() + "'s"),
            ]:
                if re.search(r"\b" + pronoun + r"\b", sent.sentence):
                    coref_map.append(CoreferenceEntry(
                        pronoun_span=pronoun.lower(),
                        antecedent=antecedent,
                        confidence=0.70,
                        resolved=True,
                    ))
                    sent.sentence = re.sub(
                        r"\b" + re.escape(pronoun) + r"\b",
                        replacement,
                        sent.sentence,
                        count=1,
                    )

    for sent in sentences:
        sent_doc = NLP(sent.sentence)
        for token in sent_doc:
            if token.text.lower() not in _PRONOUNS:
                continue
            if token.text.lower() == "it":
                continue  # handled above
            best_ant = None
            best_conf = 0.0
            for ant_text, _ in reversed(all_antecedents):
                is_entity = any(ant_text == em[0] for em in entity_mentions)
                conf = 0.85 if is_entity else 0.65
                if token.text.lower() in ("he", "him", "his") and is_entity:
                    conf = 0.9
                elif token.text.lower() in ("she", "her") and is_entity:
                    conf = 0.9
                if conf > best_conf:
                    best_conf = conf
                    best_ant = ant_text
            if best_ant and best_conf >= 0.75:
                coref_map.append(CoreferenceEntry(
                    pronoun_span=token.text,
                    antecedent=best_ant,
                    confidence=best_conf,
                    resolved=True,
                ))
                sent.sentence = re.sub(
                    r"\b" + re.escape(token.text) + r"\b",
                    best_ant,
                    sent.sentence,
                    count=1,
                )

    return sentences, coref_map


# ---------------------------------------------------------------------------
# Stage 3 — Named Entity Recognition
# ---------------------------------------------------------------------------

def stage3_ner(sentences: list[SimplifiedSentence]) -> dict[int, list[EntityInfo]]:
    """Run NER using GLiNER when available, spaCy NER as fallback."""
    results: dict[int, list[EntityInfo]] = {}

    if GLINER_AVAILABLE and GLINER_MODEL is not None:
        for idx, sent in enumerate(sentences):
            try:
                entities = GLINER_MODEL.predict_entities(
                    sent.sentence,
                    labels=_GLINER_LABELS,
                    threshold=0.5,
                )
                ents = []
                for ent in entities:
                    label = _GLINER_TO_NER.get(ent["label"])
                    if label:
                        ents.append(EntityInfo(
                            text=ent["text"],
                            label=label,
                            start_char=ent["start"],
                            end_char=ent["end"],
                        ))
                results[idx] = ents
            except Exception as exc:
                logger.warning("GLiNER failed for sentence %d: %s", idx, exc)
                results[idx] = _spacy_ner_sentence(sent.sentence)
        return results

    for idx, sent in enumerate(sentences):
        results[idx] = _spacy_ner_sentence(sent.sentence)
    return results


def _spacy_ner_sentence(sentence: str) -> list[EntityInfo]:
    doc = NLP(sentence)
    return [
        EntityInfo(
            text=ent.text,
            label=ent.label_,
            start_char=ent.start_char,
            end_char=ent.end_char,
        )
        for ent in doc.ents
        if ent.label_ in NER_TYPES
    ]


# ---------------------------------------------------------------------------
# Stage 4 — Relation Extraction (enhanced dep-parse)
# ---------------------------------------------------------------------------

def _slug(text: str) -> str:
    s = text.lower().strip()
    s = re.sub(r"[^a-z0-9]+", "-", s)
    s = s.strip("-")
    return s[:64] if s else "fact"


def _fact_id(predicate: str, arg0: str, arg1: str) -> str:
    raw = f"{predicate}-{arg0}-{arg1}"
    slug = _slug(raw)
    if not slug:
        slug = hashlib.md5(raw.encode()).hexdigest()[:12]
    return slug


def stage4_srl(
    sentences: list[SimplifiedSentence],
    entities_by_sentence: dict[int, list[EntityInfo]],
    precomputed_docs: dict[str, "Doc"] | None = None,
) -> list[ExtractedFactResult]:
    """Enhanced dep-parse RE. Handles SVO, passive voice, copular, xcomp, negation.

    precomputed_docs: unused — kept for API compatibility. Batching via nlp.pipe()
    handles the multi-sentence performance case instead.
    """
    facts: list[ExtractedFactResult] = []
    seen_ids: set[str] = set()

    # Filter sentences over the token cap before batching.
    to_parse: list[tuple[int, str]] = []  # (original_idx, text)
    for idx, sent in enumerate(sentences):
        text = sent.sentence
        # Cheap char-length pre-check before tokenising.
        if len(text) > _MAX_SENT_TOKENS * 6:
            tok_count = len(NLP.tokenizer(text))
            if tok_count > _MAX_SENT_TOKENS:
                logger.warning(
                    "stage4: sentence %d has %d tokens (cap=%d), skipping dep-parse",
                    idx, tok_count, _MAX_SENT_TOKENS,
                )
                continue
        to_parse.append((idx, text))

    # Batch all sentences through nlp.pipe() — one model call instead of N serial calls.
    parsed_docs: dict[int, "Doc"] = {}
    if to_parse:
        idxs, texts = zip(*to_parse)
        for orig_idx, doc in zip(idxs, NLP.pipe(texts)):
            parsed_docs[orig_idx] = doc

    for idx, sent in enumerate(sentences):
        doc = parsed_docs.get(idx)
        if doc is None:
            continue  # skipped (over token cap)

        ents = entities_by_sentence.get(idx, [])
        ent_type_map = {e.text: e.label for e in ents}

        for token in doc:
            if token.pos_ not in ("VERB", "AUX"):
                continue

            subject = ""
            obj = ""
            temporal = ""
            location = ""
            causal = ""
            manner = ""
            polarity = "positive"
            srl_conf = 0.70

            for child in token.children:
                dep = child.dep_

                if dep == "neg":
                    polarity = "negative"

                elif dep in ("nsubj", "nsubjpass"):
                    subject = " ".join(t.text for t in child.subtree).strip()
                    if dep == "nsubjpass":
                        srl_conf = max(srl_conf, 0.75)

                elif dep in ("dobj", "oprd"):
                    obj = " ".join(t.text for t in child.subtree).strip()

                elif dep == "attr":
                    # Copular subject complement: "The vendor is approved"
                    obj = " ".join(t.text for t in child.subtree).strip()
                    srl_conf = max(srl_conf, 0.82)

                elif dep == "acomp":
                    # Adjectival complement: "The contract remains valid"
                    obj = " ".join(t.text for t in child.subtree).strip()
                    srl_conf = max(srl_conf, 0.80)

                elif dep == "xcomp" and not obj:
                    # Open-subject control: "Alice seems ready"
                    obj = " ".join(t.text for t in child.subtree).strip()
                    srl_conf = max(srl_conf, 0.72)

                elif dep == "agent":
                    # Passive agent: "was signed by Alice" → obj=Alice
                    agent_phrase = " ".join(t.text for t in child.subtree).strip()
                    agent_phrase = re.sub(r"^by\s+", "", agent_phrase, flags=re.IGNORECASE)
                    if not obj:
                        obj = agent_phrase
                    srl_conf = max(srl_conf, 0.75)

                elif dep == "prep":
                    pobj = next((gc for gc in child.children if gc.dep_ == "pobj"), None)
                    if pobj:
                        prep_phrase = " ".join(t.text for t in child.subtree).strip()
                        prep_lower = child.text.lower()
                        if prep_lower in ("in", "at", "on", "near", "within"):
                            location = prep_phrase
                        elif prep_lower in ("after", "before", "during", "since", "until", "by"):
                            temporal = prep_phrase
                        elif prep_lower in ("because", "due"):
                            causal = prep_phrase
                        elif prep_lower in ("through", "via", "using", "with"):
                            manner = prep_phrase

            # Explicit copular enrichment for be/become/seem/appear/remain
            if token.lemma_ in _COPULAR_LEMMAS and not obj:
                for child in token.children:
                    if child.dep_ in ("attr", "acomp"):
                        obj = " ".join(t.text for t in child.subtree).strip()
                        srl_conf = max(srl_conf, 0.82)
                        break

            # Stative copular: "The vendor is approved" → (vendor, be, approved)
            # spaCy: "approved" ROOT VERB, "is" auxpass, "vendor" nsubjpass.
            # Only fires for present-tense copula (VBZ/VBP) to avoid overwriting
            # eventive passives like "was processed" which use past-tense auxpass.
            if not obj and subject:
                stative_cop = next(
                    (
                        c for c in token.children
                        if c.dep_ == "auxpass"
                        and c.lemma_ in _COPULAR_LEMMAS
                        and c.tag_ in ("VBZ", "VBP")  # present-tense only
                    ),
                    None,
                )
                if stative_cop:
                    obj = token.text.lower()
                    predicate_override = stative_cop.lemma_
                    fid = _fact_id(predicate_override, subject, obj)
                    counter = 2
                    while fid in seen_ids:
                        fid = f"{_fact_id(predicate_override, subject, obj)}-{counter}"
                        counter += 1
                    seen_ids.add(fid)
                    _et: dict[str, str] = {}
                    if subject in ent_type_map:
                        _et["subject"] = ent_type_map[subject]
                    facts.append(ExtractedFactResult(
                        id=fid,
                        subject=subject,
                        predicate=predicate_override,
                        object=obj,
                        claim=f"{subject} {predicate_override} {obj}".strip(),
                        confidence=max(srl_conf, 0.80),
                        assertion_kind="empirical",
                        source_sentence_index=idx,
                        srl_confidence=max(srl_conf, 0.80),
                        entity_types=_et,
                        depends_on=[],
                        is_root=True,
                        polarity=polarity,
                    ))
                    continue

            # Shared-object inheritance: "acquired and integrated the startup"
            # spaCy attaches the dobj to the last conj verb, leaving the head
            # verb with no obj. Peek at conj children to recover the shared obj.
            if not obj and subject:
                for conj_peek in token.children:
                    if conj_peek.dep_ == "conj" and conj_peek.pos_ in ("VERB", "AUX"):
                        for cc in conj_peek.children:
                            if cc.dep_ in ("dobj", "oprd", "attr", "acomp") and not obj:
                                obj = " ".join(t.text for t in cc.subtree).strip()

            if not subject and not obj:
                continue

            predicate = token.lemma_

            fid = _fact_id(predicate, subject, obj)
            counter = 2
            while fid in seen_ids:
                fid = f"{_fact_id(predicate, subject, obj)}-{counter}"
                counter += 1
            seen_ids.add(fid)

            entity_types: dict[str, str] = {}
            if subject in ent_type_map:
                entity_types["subject"] = ent_type_map[subject]
            if obj in ent_type_map:
                entity_types["object"] = ent_type_map[obj]

            facts.append(ExtractedFactResult(
                id=fid,
                subject=subject,
                predicate=predicate,
                object=obj,
                claim=sent.sentence,
                confidence=srl_conf,
                assertion_kind="empirical",
                modifiers=FactModifiers(
                    temporal=temporal,
                    location=location,
                    causal=causal,
                    manner=manner,
                ),
                source_sentence_index=idx,
                srl_confidence=srl_conf,
                entity_types=entity_types,
                polarity=polarity,
            ))

            # Conjoined predicates: "The firm expanded and hired employees"
            # spaCy attaches conj verbs to the ROOT but gives them no nsubj —
            # they inherit the subject from the ROOT. Emit a fact per conj verb.
            for conj in token.children:
                if conj.dep_ != "conj" or conj.pos_ not in ("VERB", "AUX"):
                    continue
                # Gather the conj verb's own object; fall back to parent's obj.
                conj_obj = ""
                conj_neg = polarity
                for cc in conj.children:
                    if cc.dep_ == "neg":
                        conj_neg = "negative"
                    elif cc.dep_ in ("dobj", "oprd"):
                        conj_obj = " ".join(t.text for t in cc.subtree).strip()
                    elif cc.dep_ == "attr":
                        conj_obj = " ".join(t.text for t in cc.subtree).strip()
                    elif cc.dep_ == "acomp" and not conj_obj:
                        conj_obj = " ".join(t.text for t in cc.subtree).strip()
                    elif cc.dep_ == "agent" and not conj_obj:
                        conj_obj = re.sub(r"^by\s+", "", " ".join(t.text for t in cc.subtree).strip(), flags=re.IGNORECASE)
                if not conj_obj:
                    conj_obj = obj  # inherit parent object (e.g. "acquired and integrated the startup")

                conj_pred = conj.lemma_
                conj_fid = _fact_id(conj_pred, subject, conj_obj)
                counter = 2
                while conj_fid in seen_ids:
                    conj_fid = f"{_fact_id(conj_pred, subject, conj_obj)}-{counter}"
                    counter += 1
                seen_ids.add(conj_fid)

                conj_et: dict[str, str] = {}
                if subject in ent_type_map:
                    conj_et["subject"] = ent_type_map[subject]
                if conj_obj in ent_type_map:
                    conj_et["object"] = ent_type_map[conj_obj]

                facts.append(ExtractedFactResult(
                    id=conj_fid,
                    subject=subject,
                    predicate=conj_pred,
                    object=conj_obj,
                    claim=sent.sentence,
                    confidence=srl_conf,
                    assertion_kind="empirical",
                    modifiers=FactModifiers(temporal=temporal, location=location, causal=causal, manner=manner),
                    source_sentence_index=idx,
                    srl_confidence=srl_conf,
                    entity_types=conj_et,
                    depends_on=[],
                    is_root=True,
                    polarity=conj_neg,
                ))

    return facts


# ---------------------------------------------------------------------------
# Stage 5 — Discourse Relation Classification & Dependency Inference
# ---------------------------------------------------------------------------

def _emit_connective_edge(
    edges: list[dict],
    rel_type: str,
    parent_id: str,
    child_id: str,
) -> None:
    if parent_id == child_id:
        return
    # causal/temporal/conditional: subordinate = premise (parent), main = conclusion (child).
    # contrastive: main → subordinate (no strong dependency direction).
    edges.append({
        "parent_id": parent_id,
        "child_id": child_id,
        "type": rel_type,
        "confidence": 0.9,
        "source": "connective",
    })


def stage5a_connective_edges(
    sentences: list[SimplifiedSentence],
    facts: list[ExtractedFactResult],
) -> list[dict]:
    """Propose connective-typed edges between facts.

    Two passes:
    1. Subordinate-clause pass: for each non-main simplified clause that contains a
       connective marker, link its facts to the co-indexed main-clause facts.
       Direction: causal/temporal/conditional → subordinate is parent (premise);
       contrastive → main is parent.
    2. Cross-sentence pass: connectives at the start of a standalone sentence
       (e.g. "However, …" / "Subsequently, …") link that sentence's facts to the
       immediately preceding sentence's facts. This handles cases like
       "The fine was issued. However, the bank appealed." where there is no
       subordinate clause — the connective is the sentence opener.
    """
    edges: list[dict] = []
    facts_by_sentence: dict[int, list[ExtractedFactResult]] = {}
    for f in facts:
        facts_by_sentence.setdefault(f.source_sentence_index, []).append(f)

    # --- Pass 1: subordinate clauses ---
    for sent_idx, sent in enumerate(sentences):
        if sent.source == "main":
            continue

        matched_rels: list[str] = []
        for phrase, rel_type in _CONNECTIVE_MAP.items():
            if _CONNECTIVE_RE[phrase].search(sent.sentence):
                matched_rels.append(rel_type)
        if not matched_rels:
            continue

        current_facts = facts_by_sentence.get(sent_idx, [])
        if not current_facts:
            continue

        for other_sent_idx, other_sent in enumerate(sentences):
            if other_sent.source != "main" or other_sent.original_index != sent.original_index:
                continue
            main_facts = facts_by_sentence.get(other_sent_idx, [])
            for rel_type in matched_rels:
                for cf in current_facts:
                    for mf in main_facts:
                        # All connective types: subordinate clause is premise (parent),
                        # main clause is conclusion (child). Contrastive ("although",
                        # "despite") follows the same pattern — the concessive clause is
                        # the contrasting premise, not the conclusion.
                        _emit_connective_edge(edges, rel_type, cf.id, mf.id)

    # --- Pass 2: sentence-opening connectives ---
    # Only consider main-source sentences (standalone, not subordinate clauses).
    main_sents = [(i, s) for i, s in enumerate(sentences) if s.source == "main"]
    for pos, (sent_idx, sent) in enumerate(main_sents):
        if pos == 0:
            continue  # no previous sentence to link to

        # Check only the opening ~60 chars — connectives appear at sentence start.
        opening = sent.sentence[:60]
        matched_rels: list[str] = []
        for phrase, rel_type in _CONNECTIVE_MAP.items():
            m = _CONNECTIVE_RE[phrase].search(opening)
            if m and m.start() < 30:  # must appear near the start
                matched_rels.append(rel_type)
        if not matched_rels:
            continue

        current_facts = facts_by_sentence.get(sent_idx, [])
        prev_sent_idx, _ = main_sents[pos - 1]
        prev_facts = facts_by_sentence.get(prev_sent_idx, [])
        if not current_facts or not prev_facts:
            continue

        for rel_type in matched_rels:
            for cf in current_facts:
                for pf in prev_facts:
                    # Previous sentence is always the premise/parent.
                    _emit_connective_edge(edges, rel_type, pf.id, cf.id)

    return edges


def _strip_det(s: str) -> str:
    """Remove leading article so 'the vendor' and 'vendor' compare equal."""
    return _DET_RE.sub("", s).strip()


def _overlap_confidence(fact_a: ExtractedFactResult, fact_b: ExtractedFactResult) -> float:
    """Higher confidence when the shared entity is a named entity (PERSON/ORG/GPE)."""
    named = {"PERSON", "ORG", "GPE"}
    a_types = set(fact_a.entity_types.values())
    b_types = set(fact_b.entity_types.values())
    if (a_types | b_types) & named:
        return 0.88
    if a_types | b_types:
        return 0.70
    return 0.45  # both sides are untyped common nouns — low signal


def _entity_qualifies(text: str, fact: ExtractedFactResult, role: str, cross_role: bool = False) -> bool:
    """Return True if the entity string is substantive enough to anchor a dependency edge.

    Same-role overlaps (subj↔subj, obj↔obj) are always accepted — even single-token
    common nouns like "board" carry signal when the same word is the subject of both facts.
    Cross-role overlaps (subj↔obj) require multi-token text or an NER-recognised entity
    to avoid noise from generic nouns like "company" or "contract".
    """
    if not cross_role:
        return True  # same-role overlap: always substantive
    if len(text.split()) >= 2:
        return True
    return role in fact.entity_types


def stage5b_entity_overlap_edges(facts: list[ExtractedFactResult]) -> list[dict]:
    """Propose edges between facts that share a normalised entity string.

    Strips leading articles, weights confidence by entity type.
    Same-role overlaps (subj↔subj, obj↔obj) always qualify; cross-role overlaps
    (subj↔obj) require multi-token text or an NER entity to suppress common-noun noise.
    """
    edges: list[dict] = []
    for i, fact_a in enumerate(facts):
        a_subj = _strip_det(fact_a.subject.lower())
        a_obj  = _strip_det(fact_a.object.lower())
        for j, fact_b in enumerate(facts):
            if i >= j:
                continue
            b_subj = _strip_det(fact_b.subject.lower())
            b_obj  = _strip_det(fact_b.object.lower())

            shared: bool = False
            # Same-role (always qualify)
            if not shared and a_subj and b_subj and (a_subj in b_subj or b_subj in a_subj):
                shared = _entity_qualifies(a_subj, fact_a, "subject", cross_role=False)
            if not shared and a_obj and b_obj and (a_obj in b_obj or b_obj in a_obj):
                shared = _entity_qualifies(a_obj, fact_a, "object", cross_role=False)
            # Cross-role (require multi-token or NER)
            if not shared and a_subj and b_obj and (a_subj in b_obj or b_obj in a_subj):
                shared = (_entity_qualifies(a_subj, fact_a, "subject", cross_role=True) or
                          _entity_qualifies(b_obj,  fact_b, "object",  cross_role=True))
            if not shared and a_obj and b_subj and (a_obj in b_subj or b_subj in a_obj):
                shared = (_entity_qualifies(a_obj,  fact_a, "object",  cross_role=True) or
                          _entity_qualifies(b_subj, fact_b, "subject", cross_role=True))

            if not shared or fact_a.id == fact_b.id:
                continue

            conf = _overlap_confidence(fact_a, fact_b)
            edges.append({
                "parent_id": fact_a.id,
                "child_id": fact_b.id,
                "type": "entity_overlap",
                "confidence": conf,
                "source": "entity_overlap",
            })
    return edges


def stage5e_chain_edges(facts: list[ExtractedFactResult]) -> list[dict]:
    """Propose directed edges where fact A's object is fact B's subject.

    "Apple acquired GitHub" → "GitHub launched Copilot": object-to-subject chaining
    is a stronger, more semantically directed dependency than a generic entity overlap.
    Confidence 0.80 — higher than common-noun overlap, lower than connective edges.
    """
    edges: list[dict] = []
    for a in facts:
        a_obj = _strip_det(a.object.lower())
        if not a_obj:
            continue
        for b in facts:
            if a.id == b.id:
                continue
            b_subj = _strip_det(b.subject.lower())
            if not b_subj:
                continue
            if a_obj in b_subj or b_subj in a_obj:
                edges.append({
                    "parent_id": a.id,
                    "child_id": b.id,
                    "type": "chain",
                    "confidence": 0.80,
                    "source": "chain",
                })
    return edges


def stage5f_coref_edges(
    coref_map: list[CoreferenceEntry],
    facts: list[ExtractedFactResult],
) -> list[dict]:
    """Propose edges grounded in coreference resolution.

    If "she → Alice" was resolved, then any fact mentioning Alice is a parent of any
    fact that originally mentioned "she". Confidence inherited from the coref model.
    These are the highest-precision edges in the pipeline.
    """
    edges: list[dict] = []
    for entry in coref_map:
        if not entry.resolved:
            continue
        ant = _strip_det(entry.antecedent.lower())
        pro = entry.pronoun_span.lower()
        for a in facts:
            a_subj = _strip_det(a.subject.lower())
            a_obj  = _strip_det(a.object.lower())
            if ant not in a_subj and ant not in a_obj:
                continue
            for b in facts:
                if a.id == b.id:
                    continue
                b_subj = _strip_det(b.subject.lower())
                b_obj  = _strip_det(b.object.lower())
                if pro in b_subj or pro in b_obj:
                    edges.append({
                        "parent_id": a.id,
                        "child_id": b.id,
                        "type": "coreference",
                        "confidence": entry.confidence,
                        "source": "coref",
                    })
    return edges


def stage5c_tms_validate(
    edges: list[dict],
    facts: list[ExtractedFactResult],
    internal_url: str,
) -> tuple[list[dict], int, int]:
    """Validate candidate edges via a single batch HTTP call.

    Replaces the original N serial /internal/validate-dependency calls.
    Falls back to local cycle detection if the batch endpoint is unavailable.
    """
    accepted: list[dict] = []
    rejected_count = 0

    fact_dicts = [
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
        for f in facts
    ]

    # Deduplicate and sort by confidence
    seen_pairs: set[tuple[str, str]] = set()
    unique_edges: list[dict] = []
    for edge in sorted(edges, key=lambda e: e["confidence"], reverse=True):
        pair = (edge["parent_id"], edge["child_id"])
        if pair not in seen_pairs and edge["parent_id"] != edge["child_id"]:
            seen_pairs.add(pair)
            unique_edges.append(edge)

    if not unique_edges:
        return [], 0, 0

    batch_payload = {
        "edges": [
            {
                "parent_id": e["parent_id"],
                "child_id": e["child_id"],
                "facts": fact_dicts,
            }
            for e in unique_edges
        ]
    }

    try:
        resp = requests.post(
            f"{internal_url}/internal/validate-dependencies-batch",
            json=batch_payload,
            timeout=5,
        )
        if resp.status_code == 200:
            results = resp.json().get("results", [])
            for edge, result in zip(unique_edges, results):
                if result.get("accepted", False):
                    accepted.append(edge)
                else:
                    rejected_count += 1
            return accepted, len(unique_edges), rejected_count
        # Fall through to local if non-200
    except Exception:
        pass

    # Local fallback (no Go server or batch endpoint not yet deployed)
    for edge in unique_edges:
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
    children_of: dict[str, set[str]] = {}
    for e in accepted:
        children_of.setdefault(e["parent_id"], set()).add(e["child_id"])
    children_of.setdefault(new_edge["parent_id"], set()).add(new_edge["child_id"])

    fact_ids = {f.id for f in facts}
    visited: set[str] = set()
    in_stack: set[str] = set()

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


def stage5d_ambiguity(
    facts: list[ExtractedFactResult],
    srl_threshold: float = 0.8,
) -> tuple[list[ExtractedFactResult], list[ConflictPair]]:
    conflict_pairs: list[ConflictPair] = []
    low_conf_groups: dict[int, list[ExtractedFactResult]] = {}
    for f in facts:
        if f.srl_confidence < srl_threshold:
            low_conf_groups.setdefault(f.source_sentence_index, []).append(f)

    for sent_idx, group in low_conf_groups.items():
        if len(group) >= 2:
            for f in group:
                f.assertion_kind = "hypothetical"
            conflict_pairs.append(ConflictPair(
                fact_a_id=group[0].id,
                fact_b_id=group[1].id,
                reason="ambiguous_parse",
            ))

    return facts, conflict_pairs


# ---------------------------------------------------------------------------
# Confidence scoring
# ---------------------------------------------------------------------------

def compute_final_confidence(
    fact: ExtractedFactResult,
    coref_confidence: float,
    entities_by_sentence: dict[int, list[EntityInfo]],
) -> float:
    ents = entities_by_sentence.get(fact.source_sentence_index, [])
    ent_texts = {e.text.lower() for e in ents}

    subject_is_entity = bool(fact.subject) and fact.subject.lower() in ent_texts
    object_is_entity = bool(fact.object) and fact.object.lower() in ent_texts

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
    """Run the full extraction pipeline."""
    text = req.text.strip()
    if not text:
        raise HTTPException(status_code=400, detail="text is required")

    internal_url = req.velarix_internal_url or VELARIX_INTERNAL_URL
    stats = ExtractionStats()

    # Run NLP once — stages 1 and 2 reuse this doc
    full_doc = NLP(text)

    # Stage 1 — clause simplification
    simplified = stage1_simplify(text, doc=full_doc)
    stats.simplified_sentences = len(simplified)

    # Stage 2 — coreference resolution
    resolved_sentences, coref_map = stage2_coreference(simplified, full_doc)
    stats.coreferences_resolved = sum(1 for c in coref_map if c.resolved)

    coref_confs = [c.confidence for c in coref_map if c.resolved]
    avg_coref_conf = sum(coref_confs) / len(coref_confs) if coref_confs else 0.85

    # Stage 3 — NER
    entities_by_sentence = stage3_ner(resolved_sentences)
    stats.entities_found = sum(len(v) for v in entities_by_sentence.values())

    # Stage 4 — relation extraction (batches all sentences through nlp.pipe())
    facts = stage4_srl(resolved_sentences, entities_by_sentence)
    stats.facts_extracted = len(facts)

    # Stage 5A — connective edges (discourse markers → typed directed edges)
    connective_edges = stage5a_connective_edges(resolved_sentences, facts)

    # Stage 5B — entity overlap edges (shared entity string → sibling link)
    overlap_edges = stage5b_entity_overlap_edges(facts)

    # Stage 5E — subject→object chaining (A's object == B's subject → chain edge)
    chain_edges = stage5e_chain_edges(facts)

    # Stage 5F — coreference-driven edges (resolved pronoun → antecedent dependency)
    coref_edges = stage5f_coref_edges(coref_map, facts)

    all_candidate_edges = connective_edges + overlap_edges + chain_edges + coref_edges

    # Stage 5C — batch TMS validation (single HTTP call)
    accepted_edges, proposed_count, rejected_count = stage5c_tms_validate(
        all_candidate_edges, facts, internal_url
    )
    stats.edges_proposed = proposed_count
    stats.edges_accepted = len(accepted_edges)
    stats.edges_rejected = rejected_count

    parent_map: dict[str, list[str]] = {}
    for edge in accepted_edges:
        parent_map.setdefault(edge["child_id"], []).append(edge["parent_id"])

    for fact in facts:
        parents = parent_map.get(fact.id, [])
        if parents:
            fact.depends_on = parents
            fact.is_root = False

    # Stage 5D — ambiguity handling
    facts, conflict_pairs = stage5d_ambiguity(facts)
    stats.ambiguous_pairs = len(conflict_pairs)

    # Final confidence scoring
    for fact in facts:
        final_conf = compute_final_confidence(fact, avg_coref_conf, entities_by_sentence)
        fact.confidence = final_conf
        if final_conf < 0.5 or fact.srl_confidence < 0.6:
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
    return {
        "status": "ok",
        "service": "delta",
        "version": "0.6.0",
        "coreferee": COREFEREE_AVAILABLE,
        "gliner": GLINER_AVAILABLE,
    }


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("SRL_SERVICE_PORT", "8090"))
    uvicorn.run(app, host="0.0.0.0", port=port)
