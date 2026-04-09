"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import dynamic from "next/dynamic";
import { apiFetch, apiUrl } from "../../../../lib/api";

const DAGExplorer = dynamic(() => import("../../../../components/DAGExplorer"), { ssr: false });

type GraphNode = {
  id: string;
  is_root?: boolean;
  status?: number;
  x?: number;
  y?: number;
  z?: number;
};

type GraphLink = {
  source: string | GraphNode;
  target: string | GraphNode;
};

type GraphData = {
  nodes: GraphNode[];
  links: GraphLink[];
};

type HistoryEvent = {
  fact?: { id?: string };
  fact_id?: string;
  timestamp: number | string;
  type: string;
};

type ImpactReport = {
  epistemic_loss?: number;
  impacted_ids?: string[];
  total_count?: number;
} | null;

type Explanation = {
  causal_chain?: Array<{ fact_id: string; tier?: string }>;
  summary?: string;
} | null;

export default function SessionView({ params }: { params: { id: string } }) {
  const router = useRouter();
  const sessionID = params.id;

  const [graphData, setGraphData] = useState<GraphData>({ nodes: [], links: [] });
  const [history, setHistory] = useState<HistoryEvent[]>([]);
  const [loading, setLoading] = useState(true);

  const [selectedFact, setSelectedFact] = useState<GraphNode | null>(null);
  const [impactReport, setImpactReport] = useState<ImpactReport>(null);
  const [explanation, setExplanation] = useState<Explanation>(null);
  const [explaining, setExplaining] = useState(false);
  const [simulating, setSimulating] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  const fetchSessionData = async () => {
    try {
      const graphRes = await apiFetch(`/v1/s/${sessionID}/graph`);
      if (graphRes.status === 401) {
        router.push("/login");
        return;
      }
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

      const historyRes = await apiFetch(`/v1/s/${sessionID}/history`);
      if (historyRes.ok) {
        const historyData = await historyRes.json();
        setHistory((historyData || []).reverse());
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchSessionData();
  }, [sessionID]);

  const handleNodeClick = async (node: GraphNode) => {
    setSelectedFact(node);
    setImpactReport(null);
    setExplanation(null);
    setSimulating(true);
    setExplaining(true);

    try {
      const [impactRes, explanationRes] = await Promise.all([
        apiFetch(`/v1/s/${sessionID}/facts/${node.id}/impact`),
        fetch(apiUrl(`/v1/s/${sessionID}/explain?fact_id=${encodeURIComponent(node.id)}`), {
          credentials: "include",
        }),
      ]);
      if (impactRes.status === 401 || explanationRes.status === 401) {
        router.push("/login");
        return;
      }

      if (impactRes.ok) {
        const impact = await impactRes.json();
        setImpactReport(impact);
      }

      if (explanationRes.ok) {
        const explanationData = await explanationRes.json();
        setExplanation(explanationData);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setSimulating(false);
      setExplaining(false);
    }
  };

  const normalizedQuery = searchQuery.trim().toLowerCase();
  const filteredNodes = graphData.nodes.filter((node) =>
    node.id.toLowerCase().includes(normalizedQuery)
  );
  const filteredGraph: GraphData = {
    nodes: filteredNodes,
    links: graphData.links.filter((link) => {
      const source = typeof link.source === "object" ? link.source.id : link.source;
      const target = typeof link.target === "object" ? link.target.id : link.target;
      const sourceVisible = filteredNodes.some((node) => node.id === source);
      const targetVisible = filteredNodes.some((node) => node.id === target);
      return sourceVisible && targetVisible;
    }),
  };

  if (loading) {
    return (
      <main className="pt-20 font-mono text-sm uppercase tracking-[0.18em] text-[var(--muted)]">
        Loading session...
      </main>
    );
  }

  return (
    <main className="pb-24 pt-8">
      <header className="flex flex-col gap-5 border-b border-[var(--line)] pb-6 md:flex-row md:items-end md:justify-between">
        <div className="space-y-2">
          <a href="/dashboard" className="text-link w-fit">
            Back to dashboard
          </a>
          <h1 className="font-display text-[clamp(2.75rem,7vw,4.5rem)] leading-[0.94] tracking-[-0.07em] break-all">
            Session {sessionID}
          </h1>
          <p className="copy-tone font-copy text-lg leading-7">
            Inspect the graph, trace the reasoning and see what breaks when a fact changes.
          </p>
        </div>
        <button onClick={fetchSessionData} className="button-ghost w-fit">
          Refresh data
        </button>
      </header>

      <div className="mt-12 grid gap-8 xl:grid-cols-[minmax(0,1.55fr)_minmax(22rem,0.9fr)] xl:items-start">
        <section className="space-y-5">
          <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
            <div className="space-y-2">
              <p className="eyebrow">Graph</p>
              <h2 className="text-2xl tracking-[-0.05em]">DAG explorer</h2>
            </div>
            <input
              type="text"
              placeholder="Search facts"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="field md:w-72"
            />
          </div>

          <div className="surface p-3 md:p-4">
            <div className="relative h-[540px] overflow-hidden border border-[var(--panel-line)] bg-black">
              <DAGExplorer
                graphData={normalizedQuery ? filteredGraph : graphData}
                onNodeClick={handleNodeClick}
              />
              <div className="pointer-events-none absolute left-4 top-4 space-y-2 bg-black/80 p-3 text-[0.72rem] font-mono uppercase tracking-[0.16em] text-white/60">
                <div className="flex items-center gap-2">
                  <span className="h-3 w-3 rounded-full bg-white" />
                  Root fact
                </div>
                <div className="flex items-center gap-2">
                  <span className="h-3 w-3 rounded-full bg-[#aaaaaa]" />
                  Derived
                </div>
                <div className="flex items-center gap-2">
                  <span className="h-3 w-3 rounded-full bg-[#333333]" />
                  Stale
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="space-y-6">
          <div className="surface space-y-5 p-6">
            <div className="space-y-2 border-b border-[var(--line)] pb-4">
              <p className="eyebrow">Inspector</p>
              <h2 className="text-2xl tracking-[-0.05em]">Causal explanation</h2>
              <p className="copy-tone font-copy text-lg leading-7">
                Select a node in the graph to inspect its reasoning and its impact on the rest of
                the session.
              </p>
            </div>

            {!selectedFact ? (
              <p className="font-copy text-lg leading-7 text-[var(--muted)]">
                No fact selected yet. Click a node in the graph to inspect it.
              </p>
            ) : (
              <div className="space-y-6">
                <div className="space-y-2">
                  <p className="field-label">Target fact</p>
                  <p className="font-mono text-sm break-all">{selectedFact.id}</p>
                </div>

                <div className="space-y-3">
                  <p className="field-label">Explanation</p>
                  {explaining ? (
                    <p className="font-mono text-sm uppercase tracking-[0.16em] text-[var(--muted)]">
                      Tracing logic...
                    </p>
                  ) : explanation ? (
                    <div className="space-y-4 border border-[var(--line)] bg-[var(--panel-soft)] p-4">
                      <p className="copy-tone font-copy text-base leading-7">
                        {explanation.summary}
                      </p>
                      {explanation.causal_chain && explanation.causal_chain.length > 0 ? (
                        <div className="space-y-2">
                          <p className="field-label">Chain of belief</p>
                          <div className="max-h-40 space-y-2 overflow-y-auto pr-1">
                            {explanation.causal_chain.map((belief, idx) => (
                              <div key={`${belief.fact_id}-${idx}`} className="border-l border-[var(--line)] pl-3">
                                <div className="font-mono text-sm break-all">{belief.fact_id}</div>
                                {belief.tier ? (
                                  <div className="mt-1 text-[0.72rem] font-mono uppercase tracking-[0.16em] text-[var(--muted)]">
                                    {belief.tier}
                                  </div>
                                ) : null}
                              </div>
                            ))}
                          </div>
                        </div>
                      ) : null}
                    </div>
                  ) : (
                    <p className="font-copy text-base leading-7 text-[var(--muted)]">
                      No explanation available for this node.
                    </p>
                  )}
                </div>

                <div className="space-y-3 border-t border-[var(--line)] pt-5">
                  <p className="field-label">Impact simulation</p>
                  {simulating ? (
                    <p className="font-mono text-sm uppercase tracking-[0.16em] text-[var(--muted)]">
                      Running simulation...
                    </p>
                  ) : impactReport ? (
                    <div className="space-y-4 border border-[var(--line)] bg-[var(--panel-soft)] p-4">
                      <div className="grid gap-4 sm:grid-cols-2">
                        <div>
                          <p className="field-label">Total impact</p>
                          <p className="mt-2 text-3xl tracking-[-0.06em]">
                            {impactReport.total_count || 0}
                          </p>
                        </div>
                        <div>
                          <p className="field-label">Epistemic loss</p>
                          <p className="mt-2 text-3xl tracking-[-0.06em]">
                            {(impactReport.epistemic_loss || 0).toFixed(2)}
                          </p>
                        </div>
                      </div>
                      {impactReport.impacted_ids && impactReport.impacted_ids.length > 0 ? (
                        <div className="space-y-2">
                          <p className="field-label">Cascading failures</p>
                          <div className="max-h-40 space-y-2 overflow-y-auto pr-1">
                            {impactReport.impacted_ids.map((id, idx) => (
                              <div key={`${id}-${idx}`} className="border-l border-[var(--line)] pl-3">
                                <div className="font-mono text-sm break-all">{id}</div>
                              </div>
                            ))}
                          </div>
                        </div>
                      ) : null}
                    </div>
                  ) : (
                    <p className="font-copy text-base leading-7 text-[var(--muted)]">
                      No impact report available yet.
                    </p>
                  )}
                </div>
              </div>
            )}
          </div>

          <div className="surface flex h-[540px] flex-col gap-4 p-6">
            <div className="space-y-2 border-b border-[var(--line)] pb-4">
              <p className="eyebrow">Journal</p>
              <h2 className="text-2xl tracking-[-0.05em]">Audit log</h2>
            </div>
            <div className="flex-1 space-y-3 overflow-y-auto pr-1">
              {history.length === 0 ? (
                <p className="font-copy text-lg leading-7 text-[var(--muted)]">
                  No events found in this session.
                </p>
              ) : (
                history.map((event, idx) => (
                  <div key={`${event.type}-${event.timestamp}-${idx}`} className="border-b border-[var(--line)] pb-3 last:border-b-0">
                    <div className="flex items-start justify-between gap-4">
                      <span className="font-mono text-[0.72rem] uppercase tracking-[0.18em]">
                        {event.type.replace("event_", "")}
                      </span>
                      <span className="font-mono text-[0.72rem] uppercase tracking-[0.16em] text-[var(--muted)]">
                        {new Date(event.timestamp).toLocaleTimeString()}
                      </span>
                    </div>
                    {event.fact_id ? (
                      <div className="mt-2 font-mono text-sm break-all text-[var(--muted)]">
                        Fact: {event.fact_id}
                      </div>
                    ) : null}
                    {event.fact?.id ? (
                      <div className="mt-2 font-mono text-sm break-all text-[var(--muted)]">
                        Fact: {event.fact.id}
                      </div>
                    ) : null}
                  </div>
                ))
              )}
            </div>
          </div>
        </section>
      </div>
    </main>
  );
}
