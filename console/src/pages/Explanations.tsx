import { useEffect, useState, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Brain, Clock, GitBranch, Shield, AlertTriangle, CheckCircle2, XCircle, Loader2, ChevronDown, Search } from 'lucide-react';

const API_BASE = 'http://localhost:8080';

interface BeliefExplanation {
  fact_id: string;
  confidence: number;
  tier: 'certain' | 'probable' | 'uncertain';
  provenance?: Record<string, any>;
  payload?: Record<string, any>;
  is_root: boolean;
  parents?: string[];
}

interface CounterfactualResult {
  removed_fact_id: string;
  impacted_facts: string[];
  direct_count: number;
  total_count: number;
  epistemic_loss: number;
  narrative: string;
}

interface ExplanationOutput {
  fact_id: string;
  session_id: string;
  timestamp: number;
  causal_chain: BeliefExplanation[];
  counterfactual?: CounterfactualResult;
}

interface StoredExplanation {
  session_id: string;
  timestamp: number;
  content: ExplanationOutput;
  content_hash: string;
  tampered: boolean;
}

interface SessionInfo {
  id: string;
}

interface FactInfo {
  id: string;
  is_root: boolean;
  derived_status: number;
  resolved_status: number;
  payload?: Record<string, any>;
}

const tierColors: Record<string, { bg: string; text: string; border: string; glow: string }> = {
  certain: { bg: 'bg-emerald-500/10', text: 'text-emerald-400', border: 'border-emerald-500/30', glow: 'shadow-[0_0_12px_rgba(16,185,129,0.3)]' },
  probable: { bg: 'bg-amber-500/10', text: 'text-amber-400', border: 'border-amber-500/30', glow: 'shadow-[0_0_12px_rgba(245,158,11,0.3)]' },
  uncertain: { bg: 'bg-red-500/10', text: 'text-red-400', border: 'border-red-500/30', glow: 'shadow-[0_0_12px_rgba(239,68,68,0.3)]' },
};

export default function Explanations() {
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [selectedSession, setSelectedSession] = useState('');
  const [facts, setFacts] = useState<FactInfo[]>([]);
  const [selectedFact, setSelectedFact] = useState('');
  const [counterfactualFact, setCounterfactualFact] = useState('');
  const [explanation, setExplanation] = useState<ExplanationOutput | null>(null);
  const [storedExplanations, setStoredExplanations] = useState<StoredExplanation[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [historyTimestamp, setHistoryTimestamp] = useState<number | null>(null);
  const [historyRange, setHistoryRange] = useState<{ min: number; max: number }>({ min: 0, max: Date.now() });
  const [showSessionDropdown, setShowSessionDropdown] = useState(false);

  const token = localStorage.getItem('velarix_token') || '';
  const headers = { 'Authorization': `Bearer ${token}` };

  // Fetch sessions
  useEffect(() => {
    fetch(`${API_BASE}/v1/sessions`, { headers })
      .then(r => r.json())
      .then(data => { if (Array.isArray(data)) setSessions(data); })
      .catch(() => {});
  }, []);

  // Fetch facts when session changes
  useEffect(() => {
    if (!selectedSession) return;
    setFacts([]);
    setSelectedFact('');
    setExplanation(null);

    fetch(`${API_BASE}/v1/s/${selectedSession}/facts`, { headers })
      .then(r => r.json())
      .then(data => {
        if (Array.isArray(data)) {
          setFacts(data);
        }
      })
      .catch(() => {});

    // Fetch history range for the timestamp scrubber
    fetch(`${API_BASE}/v1/s/${selectedSession}/history`, { headers })
      .then(r => r.json())
      .then(data => {
        if (Array.isArray(data) && data.length > 0) {
          const timestamps = data.map((e: any) => e.timestamp);
          setHistoryRange({
            min: Math.min(...timestamps),
            max: Math.max(...timestamps),
          });
        }
      })
      .catch(() => {});

    // Fetch stored explanations
    fetch(`${API_BASE}/v1/s/${selectedSession}/explanations`, { headers })
      .then(r => r.json())
      .then(data => { if (Array.isArray(data)) setStoredExplanations(data); })
      .catch(() => {});
  }, [selectedSession]);

  const fetchExplanation = useCallback(async () => {
    if (!selectedSession) return;
    setLoading(true);
    setError('');
    setExplanation(null);

    const params = new URLSearchParams();
    if (selectedFact) params.set('fact_id', selectedFact);
    if (historyTimestamp) params.set('timestamp', new Date(historyTimestamp).toISOString());
    if (counterfactualFact) params.set('counterfactual_fact_id', counterfactualFact);

    try {
      const resp = await fetch(`${API_BASE}/v1/s/${selectedSession}/explain?${params.toString()}`, { headers });
      if (!resp.ok) {
        const text = await resp.text();
        setError(text || 'Failed to generate explanation');
        return;
      }
      const data = await resp.json();
      setExplanation(data);

      // Refresh stored explanations
      const stResp = await fetch(`${API_BASE}/v1/s/${selectedSession}/explanations`, { headers });
      if (stResp.ok) {
        const stData = await stResp.json();
        if (Array.isArray(stData)) setStoredExplanations(stData);
      }
    } catch (e: any) {
      setError(e.message || 'Network error');
    } finally {
      setLoading(false);
    }
  }, [selectedSession, selectedFact, historyTimestamp, counterfactualFact]);

  return (
    <div className="max-w-6xl mx-auto space-y-8 animate-in fade-in duration-500">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-display font-semibold text-white flex items-center gap-3">
            <Brain className="w-7 h-7 text-v-accent" />
            Explanation Panel
          </h1>
          <p className="text-sm text-v-text-muted mt-1">
            Inspect confidence-weighted, provenance-backed reasoning for any belief.
          </p>
        </div>
      </div>

      {/* Controls */}
      <div className="glass-panel rounded-xl p-6 space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {/* Session Selector */}
          <div className="space-y-2">
            <label className="text-xs font-mono text-v-text-muted uppercase tracking-wider">Session</label>
            <div className="relative">
              <button
                onClick={() => setShowSessionDropdown(!showSessionDropdown)}
                className="w-full bg-v-bg border border-v-border rounded-lg px-4 py-2.5 text-sm text-white text-left flex items-center justify-between hover:border-v-accent/50 transition-colors"
                id="session-selector"
              >
                <span className={selectedSession ? 'text-white' : 'text-v-text-muted'}>
                  {selectedSession || 'Select session...'}
                </span>
                <ChevronDown className="w-4 h-4 text-v-text-muted" />
              </button>
              <AnimatePresence>
                {showSessionDropdown && (
                  <motion.div
                    initial={{ opacity: 0, y: -5 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -5 }}
                    className="absolute z-50 top-full mt-1 w-full bg-[#0a0a0a] border border-v-border rounded-lg shadow-2xl max-h-48 overflow-y-auto"
                  >
                    {sessions.length === 0 ? (
                      <div className="px-4 py-3 text-sm text-v-text-muted">No sessions found</div>
                    ) : sessions.map(s => (
                      <button
                        key={s.id}
                        onClick={() => { setSelectedSession(s.id); setShowSessionDropdown(false); }}
                        className="w-full text-left px-4 py-2 text-sm text-v-text-muted hover:text-white hover:bg-white/5 transition-colors"
                      >
                        {s.id}
                      </button>
                    ))}
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          </div>

          {/* Fact Selector */}
          <div className="space-y-2">
            <label className="text-xs font-mono text-v-text-muted uppercase tracking-wider">Fact ID</label>
            <select
              value={selectedFact}
              onChange={e => setSelectedFact(e.target.value)}
              className="w-full bg-v-bg border border-v-border rounded-lg px-4 py-2.5 text-sm text-white focus:outline-none focus:border-v-accent/50 transition-colors"
              id="fact-selector"
            >
              <option value="">All / Latest</option>
              {facts.map(f => (
                <option key={f.id} value={f.id}>{f.id} ({f.is_root ? 'root' : 'derived'})</option>
              ))}
            </select>
          </div>

          {/* Counterfactual Input */}
          <div className="space-y-2">
            <label className="text-xs font-mono text-v-text-muted uppercase tracking-wider">Counterfactual</label>
            <select
              value={counterfactualFact}
              onChange={e => setCounterfactualFact(e.target.value)}
              className="w-full bg-v-bg border border-v-border rounded-lg px-4 py-2.5 text-sm text-white focus:outline-none focus:border-v-accent/50 transition-colors"
              id="counterfactual-selector"
            >
              <option value="">None</option>
              {facts.map(f => (
                <option key={f.id} value={f.id}>{f.id}</option>
              ))}
            </select>
          </div>
        </div>

        {/* Timestamp Scrubber */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <label className="text-xs font-mono text-v-text-muted uppercase tracking-wider flex items-center gap-2">
              <Clock className="w-3.5 h-3.5" />
              Time Travel
            </label>
            <span className="text-xs font-mono text-v-text-muted">
              {historyTimestamp ? new Date(historyTimestamp).toLocaleString() : 'Current state'}
            </span>
          </div>
          <input
            type="range"
            min={historyRange.min}
            max={historyRange.max}
            value={historyTimestamp || historyRange.max}
            onChange={e => {
              const val = parseInt(e.target.value);
              setHistoryTimestamp(val === historyRange.max ? null : val);
            }}
            className="w-full h-1.5 bg-v-border rounded-lg appearance-none cursor-pointer accent-v-accent"
            id="timestamp-scrubber"
          />
          <div className="flex justify-between text-[10px] font-mono text-v-text-muted/60">
            <span>{new Date(historyRange.min).toLocaleTimeString()}</span>
            <span>{new Date(historyRange.max).toLocaleTimeString()}</span>
          </div>
        </div>

        {/* Generate Button */}
        <button
          onClick={fetchExplanation}
          disabled={!selectedSession || loading}
          className="w-full py-3 bg-gradient-to-r from-v-accent to-emerald-500 text-white font-medium rounded-lg hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed flex items-center justify-center gap-2 text-sm"
          id="generate-explanation"
        >
          {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Brain className="w-4 h-4" />}
          {loading ? 'Generating...' : 'Generate Explanation'}
        </button>

        {error && (
          <div className="flex items-center gap-2 text-red-400 text-sm bg-red-500/10 border border-red-500/20 px-4 py-2 rounded-lg">
            <XCircle className="w-4 h-4 shrink-0" />
            {error}
          </div>
        )}
      </div>

      {/* Explanation Display */}
      <AnimatePresence mode="wait">
        {explanation && (
          <motion.div
            key={explanation.timestamp}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            className="space-y-6"
          >
            {/* Causal Chain */}
            <div className="glass-panel rounded-xl p-6">
              <h2 className="text-lg font-medium text-white mb-1 flex items-center gap-2">
                <GitBranch className="w-5 h-5 text-v-accent" />
                Causal Chain for <code className="text-v-accent ml-1">{explanation.fact_id}</code>
              </h2>
              <p className="text-xs text-v-text-muted mb-6">
                Generated at {new Date(explanation.timestamp).toLocaleString()}
              </p>

              <div className="space-y-3">
                {explanation.causal_chain.map((belief, idx) => {
                  const colors = tierColors[belief.tier] || tierColors.uncertain;
                  return (
                    <motion.div
                      key={belief.fact_id}
                      initial={{ opacity: 0, x: -20 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ delay: idx * 0.05 }}
                      className={`border ${colors.border} ${colors.bg} rounded-lg p-4 ${colors.glow}`}
                      id={`belief-${belief.fact_id}`}
                    >
                      <div className="flex items-start justify-between mb-2">
                        <div className="flex items-center gap-2">
                          {belief.tier === 'certain' && <CheckCircle2 className={`w-4 h-4 ${colors.text}`} />}
                          {belief.tier === 'probable' && <AlertTriangle className={`w-4 h-4 ${colors.text}`} />}
                          {belief.tier === 'uncertain' && <XCircle className={`w-4 h-4 ${colors.text}`} />}
                          <span className="text-white font-medium text-sm">{belief.fact_id}</span>
                          {belief.is_root && (
                            <span className="text-[10px] bg-white/10 text-v-text-muted px-2 py-0.5 rounded-full uppercase tracking-wider">root</span>
                          )}
                        </div>
                        <div className="flex items-center gap-2">
                          <span className={`text-xs font-mono ${colors.text} px-2 py-0.5 ${colors.bg} rounded-full border ${colors.border}`}>
                            {belief.tier} · {(belief.confidence * 100).toFixed(0)}%
                          </span>
                        </div>
                      </div>

                      {belief.parents && belief.parents.length > 0 && (
                        <div className="text-xs text-v-text-muted mb-2">
                          <span className="opacity-60">Depends on:</span>{' '}
                          {belief.parents.map((p, i) => (
                            <span key={p}>
                              <code className="text-purple-400">{p}</code>
                              {i < belief.parents!.length - 1 ? ', ' : ''}
                            </span>
                          ))}
                        </div>
                      )}

                      {belief.payload && Object.keys(belief.payload).filter(k => k !== '_provenance').length > 0 && (
                        <div className="text-xs text-v-text-muted bg-black/20 rounded p-2 font-mono mt-2 overflow-x-auto">
                          {JSON.stringify(Object.fromEntries(Object.entries(belief.payload).filter(([k]) => k !== '_provenance')), null, 2)}
                        </div>
                      )}

                      {/* Provenance */}
                      {belief.provenance && (
                        <div className="mt-2 pt-2 border-t border-white/5">
                          <div className="text-[10px] text-v-text-muted/70 uppercase tracking-wider mb-1">Provenance</div>
                          <div className="flex flex-wrap gap-2 text-[11px]">
                            {belief.provenance.source && (
                              <span className="bg-white/5 px-2 py-0.5 rounded text-v-text-muted">source: <span className="text-white">{belief.provenance.source}</span></span>
                            )}
                            {belief.provenance.model && (
                              <span className="bg-white/5 px-2 py-0.5 rounded text-v-text-muted">model: <span className="text-white">{belief.provenance.model}</span></span>
                            )}
                            {belief.provenance.tool_call_id && (
                              <span className="bg-white/5 px-2 py-0.5 rounded text-v-text-muted">tool_call: <span className="text-white">{belief.provenance.tool_call_id}</span></span>
                            )}
                          </div>
                        </div>
                      )}
                    </motion.div>
                  );
                })}
              </div>
            </div>

            {/* Counterfactual */}
            {explanation.counterfactual && (
              <div className="glass-panel rounded-xl p-6 border-l-4 border-purple-500/50">
                <h2 className="text-lg font-medium text-white mb-4 flex items-center gap-2">
                  <GitBranch className="w-5 h-5 text-purple-400" />
                  Counterfactual Analysis
                </h2>
                <div className="bg-purple-500/5 border border-purple-500/20 rounded-lg p-4 mb-4">
                  <p className="text-sm text-purple-200 leading-relaxed italic">
                    "{explanation.counterfactual.narrative}"
                  </p>
                </div>
                <div className="grid grid-cols-3 gap-4 text-center">
                  <div className="bg-white/5 rounded-lg p-3">
                    <div className="text-2xl font-display font-semibold text-white">{explanation.counterfactual.total_count}</div>
                    <div className="text-xs text-v-text-muted mt-1">Total Impacted</div>
                  </div>
                  <div className="bg-white/5 rounded-lg p-3">
                    <div className="text-2xl font-display font-semibold text-white">{explanation.counterfactual.direct_count}</div>
                    <div className="text-xs text-v-text-muted mt-1">Direct Children</div>
                  </div>
                  <div className="bg-white/5 rounded-lg p-3">
                    <div className="text-2xl font-display font-semibold text-red-400">{explanation.counterfactual.epistemic_loss.toFixed(2)}</div>
                    <div className="text-xs text-v-text-muted mt-1">Epistemic Loss</div>
                  </div>
                </div>
                {explanation.counterfactual.impacted_facts.length > 0 && (
                  <div className="mt-4">
                    <div className="text-xs text-v-text-muted uppercase tracking-wider mb-2">Impacted Facts</div>
                    <div className="flex flex-wrap gap-1.5">
                      {explanation.counterfactual.impacted_facts.map(f => (
                        <span key={f} className="text-xs bg-red-500/10 text-red-400 border border-red-500/20 px-2 py-0.5 rounded-full font-mono">{f}</span>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Stored Explanations */}
      {storedExplanations.length > 0 && (
        <div className="glass-panel rounded-xl p-6">
          <h2 className="text-lg font-medium text-white mb-4 flex items-center gap-2">
            <Shield className="w-5 h-5 text-v-accent" />
            Explanation Audit Trail
          </h2>
          <div className="space-y-2">
            {storedExplanations.map((record, idx) => (
              <div
                key={idx}
                className={`flex items-center justify-between px-4 py-3 rounded-lg border ${
                  record.tampered
                    ? 'border-red-500/30 bg-red-500/5'
                    : 'border-v-border bg-white/[0.02]'
                } hover:bg-white/5 transition-colors group`}
              >
                <div className="flex items-center gap-3">
                  {record.tampered ? (
                    <XCircle className="w-4 h-4 text-red-400" />
                  ) : (
                    <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                  )}
                  <div>
                    <div className="text-sm text-white">
                      {record.content?.fact_id || 'Explanation'}
                      {record.tampered && (
                        <span className="ml-2 text-[10px] bg-red-500/20 text-red-400 px-2 py-0.5 rounded-full uppercase tracking-wider">
                          tampered
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-v-text-muted font-mono mt-0.5">
                      {new Date(record.timestamp).toLocaleString()}
                    </div>
                  </div>
                </div>
                <div className="text-[10px] font-mono text-v-text-muted/50 group-hover:text-v-text-muted transition-colors truncate max-w-[200px]">
                  SHA: {record.content_hash?.substring(0, 16)}...
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
