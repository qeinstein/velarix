"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import dynamic from "next/dynamic";
import { apiFetch, apiUrl } from "@/lib/api";

const DAGExplorer = dynamic(() => import("@/components/DAGExplorer"), { ssr: false });

type GraphNode = { id: string; is_root?: boolean; status?: number; x?: number; y?: number; z?: number };
type GraphLink = { source: string | GraphNode; target: string | GraphNode };
type GraphData = { nodes: GraphNode[]; links: GraphLink[] };
type HistoryEvent = { fact?: { id?: string }; fact_id?: string; timestamp: number | string; type: string };
type ImpactReport = { epistemic_loss?: number; impacted_ids?: string[]; total_count?: number } | null;
type Explanation = { causal_chain?: Array<{ fact_id: string; tier?: string }>; summary?: string } | null;
type SessionSummary = { id: string; name?: string; fact_count?: number; enforcement_mode?: string };

export default function SessionView({ params }: { params: { id: string } }) {
  const router = useRouter();
  const sessionID = params.id;

  const [summary, setSummary] = useState<SessionSummary | null>(null);
  const [graphData, setGraphData] = useState<GraphData>({ nodes: [], links: [] });
  const [history, setHistory] = useState<HistoryEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedFact, setSelectedFact] = useState<GraphNode | null>(null);
  const [impactReport, setImpactReport] = useState<ImpactReport>(null);
  const [explanation, setExplanation] = useState<Explanation>(null);
  const [explaining, setExplaining] = useState(false);
  const [simulating, setSimulating] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeTab, setActiveTab] = useState<"graph" | "history">("graph");

  const fetchSessionData = async () => {
    try {
      const [graphRes, historyRes, summaryRes] = await Promise.all([
        apiFetch(`/v1/s/${sessionID}/graph`),
        apiFetch(`/v1/s/${sessionID}/history`),
        apiFetch(`/v1/s/${sessionID}/summary`),
      ]);
      if (graphRes.status === 401) { router.push("/login"); return; }

      if (graphRes.ok) {
        const data = await graphRes.json();
        setGraphData({
          nodes: data.nodes || [],
          links: (data.edges || []).map((edge: { from: string; to: string }) => ({
            source: edge.from,
            target: edge.to,
          })),
        });
      }
      if (historyRes.ok) {
        const historyData = await historyRes.json();
        setHistory((historyData || []).reverse());
      }
      if (summaryRes.ok) {
        setSummary(await summaryRes.json());
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void fetchSessionData(); }, [sessionID]);

  const handleNodeClick = async (node: GraphNode) => {
    setSelectedFact(node);
    setImpactReport(null);
    setExplanation(null);
    setSimulating(true);
    setExplaining(true);
    try {
      const [impactRes, explanationRes] = await Promise.all([
        apiFetch(`/v1/s/${sessionID}/facts/${node.id}/impact`),
        fetch(apiUrl(`/v1/s/${sessionID}/explain?fact_id=${encodeURIComponent(node.id)}`), { credentials: "include" }),
      ]);
      if (impactRes.status === 401 || explanationRes.status === 401) { router.push("/login"); return; }
      if (impactRes.ok) setImpactReport(await impactRes.json());
      if (explanationRes.ok) setExplanation(await explanationRes.json());
    } catch (err) {
      console.error(err);
    } finally {
      setSimulating(false);
      setExplaining(false);
    }
  };

  const normalizedQuery = searchQuery.trim().toLowerCase();
  const filteredNodes = graphData.nodes.filter((n) => n.id.toLowerCase().includes(normalizedQuery));
  const filteredGraph: GraphData = {
    nodes: filteredNodes,
    links: graphData.links.filter((link) => {
      const src = typeof link.source === "object" ? link.source.id : link.source;
      const tgt = typeof link.target === "object" ? link.target.id : link.target;
      return filteredNodes.some((n) => n.id === src) && filteredNodes.some((n) => n.id === tgt);
    }),
  };

  const rootCount = graphData.nodes.filter((n) => n.is_root).length;
  const derivedCount = graphData.nodes.filter((n) => !n.is_root && (n.status ?? 0) >= 0.6).length;
  const staleCount = graphData.nodes.filter((n) => !n.is_root && (n.status ?? 0) < 0.6).length;

  if (loading) {
    return (
      <div className="pt-20 font-mono text-sm uppercase tracking-[0.18em] text-[var(--muted)]">
        Loading session...
      </div>
    );
  }

  return (
    <>
      {/* Header */}
      <div className="dash-page-header">
        <a
          href="/dashboard/projects"
          className="font-mono text-[0.72rem] uppercase tracking-[0.14em] text-[var(--muted)] transition-colors hover:text-[var(--foreground)]"
        >
          ← Projects
        </a>
        <div className="dash-page-header-row">
          <div>
            <h1 className="dash-page-title">
              {summary?.name || sessionID}
            </h1>
            {summary?.name && (
              <p className="mt-1 font-mono text-[0.78rem] tracking-[0.08em] text-[var(--muted)]">
                {sessionID}
              </p>
            )}
            <p className="dash-page-subtitle">
              {summary?.enforcement_mode
                ? `Enforcement: ${summary.enforcement_mode}`
                : "Inspect the graph, trace reasoning, and see what breaks when a fact changes."}
            </p>
          </div>
          <button onClick={fetchSessionData} className="button-ghost">
            Refresh
          </button>
        </div>
      </div>

      {/* Stats bar */}
      <div className="dash-stat-grid mb-8">
        <div className="dash-stat-card">
          <p className="field-label">Total facts</p>
          <p className="metric-value mt-3">{graphData.nodes.length}</p>
        </div>
        <div className="dash-stat-card">
          <p className="field-label">Root facts</p>
          <p className="metric-value mt-3">{rootCount}</p>
        </div>
        <div className="dash-stat-card">
          <p className="field-label">Derived</p>
          <p className="metric-value mt-3">{derivedCount}</p>
        </div>
        <div className="dash-stat-card">
          <p className="field-label">Stale</p>
          <p className="metric-value mt-3">{staleCount}</p>
        </div>
        <div className="dash-stat-card">
          <p className="field-label">Events</p>
          <p className="metric-value mt-3">{history.length}</p>
        </div>
      </div>

      {/* Main content */}
      <div className="grid gap-8 xl:grid-cols-[minmax(0,1.6fr)_minmax(20rem,0.85fr)] xl:items-start">
        {/* Left: graph */}
        <section className="space-y-5">
          <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
            <div className="space-y-1">
              <p className="eyebrow">Causal graph</p>
              <h2 className="text-2xl tracking-[-0.05em]">DAG explorer</h2>
            </div>
            <input
              type="text"
              placeholder="Search facts"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="field md:w-64"
            />
          </div>

          <div className="surface p-3">
            <div className="relative h-[540px] overflow-hidden border border-[var(--panel-line)] bg-black">
              <DAGExplorer
                graphData={normalizedQuery ? filteredGraph : graphData}
                onNodeClick={handleNodeClick}
              />
              <div className="pointer-events-none absolute left-4 top-4 space-y-2 bg-black/80 p-3 text-[0.72rem] font-mono uppercase tracking-[0.16em] text-white/60">
                <div className="flex items-center gap-2">
                  <span className="h-3 w-3 rounded-full bg-white" />Root fact
                </div>
                <div className="flex items-center gap-2">
                  <span className="h-3 w-3 rounded-full bg-[#aaaaaa]" />Derived
                </div>
                <div className="flex items-center gap-2">
                  <span className="h-3 w-3 rounded-full bg-[#444444]" />Stale
                </div>
              </div>
              {selectedFact && (
                <div className="pointer-events-none absolute bottom-4 left-4 bg-black/80 p-2 font-mono text-[0.7rem] tracking-[0.1em] text-white/70">
                  Selected: {selectedFact.id}
                </div>
              )}
            </div>
          </div>
        </section>

        {/* Right: inspector + audit */}
        <section className="space-y-6">
          {/* Inspector */}
          <div className="surface space-y-5 p-6">
            <div className="space-y-1 border-b border-[var(--line)] pb-4">
              <p className="eyebrow">Inspector</p>
              <h2 className="text-2xl tracking-[-0.05em]">Causal explanation</h2>
              <p className="copy-tone font-copy text-base leading-7">
                Click a node to trace its reasoning and simulate its impact.
              </p>
            </div>

            {!selectedFact ? (
              <p className="font-copy text-base leading-7 text-[var(--muted)]">
                No fact selected yet.
              </p>
            ) : (
              <div className="space-y-5">
                <div className="space-y-1">
                  <p className="field-label">Selected fact</p>
                  <p className="break-all font-mono text-sm">{selectedFact.id}</p>
                  <div className="flex gap-3 pt-1">
                    <span className="status-pill">
                      {selectedFact.is_root ? "Root" : "Derived"}
                    </span>
                    <span className="status-pill">
                      Status: {(selectedFact.status ?? 0).toFixed(2)}
                    </span>
                  </div>
                </div>

                {/* Explanation */}
                <div className="space-y-2">
                  <p className="field-label">Explanation</p>
                  {explaining ? (
                    <p className="font-mono text-sm uppercase tracking-[0.14em] text-[var(--muted)]">
                      Tracing logic...
                    </p>
                  ) : explanation ? (
                    <div className="space-y-3 border border-[var(--line)] bg-[var(--panel-soft)] p-4">
                      <p className="copy-tone font-copy text-base leading-7">{explanation.summary}</p>
                      {explanation.causal_chain && explanation.causal_chain.length > 0 && (
                        <div className="space-y-2">
                          <p className="field-label">Chain of belief</p>
                          <div className="max-h-36 space-y-2 overflow-y-auto pr-1">
                            {explanation.causal_chain.map((belief, idx) => (
                              <div key={`${belief.fact_id}-${idx}`} className="border-l border-[var(--line)] pl-3">
                                <div className="break-all font-mono text-sm">{belief.fact_id}</div>
                                {belief.tier && (
                                  <div className="mt-0.5 font-mono text-[0.68rem] uppercase tracking-[0.14em] text-[var(--muted)]">
                                    {belief.tier}
                                  </div>
                                )}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  ) : (
                    <p className="font-copy text-sm text-[var(--muted)]">No explanation available.</p>
                  )}
                </div>

                {/* Impact */}
                <div className="space-y-2 border-t border-[var(--line)] pt-4">
                  <p className="field-label">Impact simulation</p>
                  {simulating ? (
                    <p className="font-mono text-sm uppercase tracking-[0.14em] text-[var(--muted)]">
                      Running simulation...
                    </p>
                  ) : impactReport ? (
                    <div className="space-y-3 border border-[var(--line)] bg-[var(--panel-soft)] p-4">
                      <div className="grid gap-3 sm:grid-cols-2">
                        <div>
                          <p className="field-label">Impacted facts</p>
                          <p className="mt-1 text-2xl tracking-[-0.05em]">
                            {impactReport.total_count || 0}
                          </p>
                        </div>
                        <div>
                          <p className="field-label">Epistemic loss</p>
                          <p className="mt-1 text-2xl tracking-[-0.05em]">
                            {(impactReport.epistemic_loss || 0).toFixed(3)}
                          </p>
                        </div>
                      </div>
                      {impactReport.impacted_ids && impactReport.impacted_ids.length > 0 && (
                        <div className="space-y-1">
                          <p className="field-label">Cascading failures</p>
                          <div className="max-h-32 space-y-1 overflow-y-auto pr-1">
                            {impactReport.impacted_ids.map((id, idx) => (
                              <div key={`${id}-${idx}`} className="border-l border-[var(--line)] pl-3 font-mono text-sm break-all text-[var(--muted)]">
                                {id}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  ) : (
                    <p className="font-copy text-sm text-[var(--muted)]">No impact data yet.</p>
                  )}
                </div>
              </div>
            )}
          </div>

          {/* Audit log */}
          <div className="surface flex max-h-[480px] flex-col gap-4 p-6">
            <div className="space-y-1 border-b border-[var(--line)] pb-4">
              <p className="eyebrow">Journal</p>
              <h2 className="text-2xl tracking-[-0.05em]">Audit log</h2>
            </div>
            <div className="flex-1 space-y-3 overflow-y-auto pr-1">
              {history.length === 0 ? (
                <p className="font-copy text-base leading-7 text-[var(--muted)]">
                  No events in this session.
                </p>
              ) : (
                history.map((event, idx) => (
                  <div
                    key={`${event.type}-${event.timestamp}-${idx}`}
                    className="border-b border-[var(--line)] pb-2 last:border-b-0"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <span className="font-mono text-[0.7rem] uppercase tracking-[0.18em]">
                        {event.type.replace("event_", "")}
                      </span>
                      <span className="flex-shrink-0 font-mono text-[0.7rem] tracking-[0.1em] text-[var(--muted)]">
                        {new Date(event.timestamp).toLocaleTimeString()}
                      </span>
                    </div>
                    {(event.fact_id || event.fact?.id) && (
                      <div className="mt-1 break-all font-mono text-[0.72rem] text-[var(--muted)]">
                        {event.fact_id || event.fact?.id}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>
        </section>
      </div>
    </>
  );
}
