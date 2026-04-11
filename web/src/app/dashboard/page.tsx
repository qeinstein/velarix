"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api";

type Session = {
  id: string;
  name?: string;
  description?: string;
  fact_count?: number;
  last_activity_at?: number;
  created_at?: number;
};

type Usage = {
  api_requests?: number;
  facts_asserted?: number;
  facts_pruned?: number;
  logic_prunings?: number;
  sessions_created?: number;
};

function formatRelative(ms: number) {
  if (!ms) return "—";
  const diff = Date.now() - ms;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export default function DashboardOverview() {
  const router = useRouter();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [usage, setUsage] = useState<Usage>({});
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");

  const fetchData = async () => {
    try {
      const [sessRes, usageRes] = await Promise.all([
        apiFetch("/v1/org/sessions?limit=50"),
        apiFetch("/v1/org/usage"),
      ]);
      if (sessRes.status === 401) {
        router.push("/login");
        return;
      }
      if (sessRes.ok) {
        const data = await sessRes.json();
        setSessions(data?.items || []);
      }
      if (usageRes.ok) {
        setUsage((await usageRes.json()) || {});
      }
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchData();
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    try {
      const res = await apiFetch("/v1/org/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: newName.trim(), description: newDesc.trim() }),
      });
      if (res.ok) {
        const created = await res.json();
        setShowCreate(false);
        setNewName("");
        setNewDesc("");
        router.push(`/dashboard/session/${created.id}`);
      }
    } finally {
      setCreating(false);
    }
  };

  const prunedCount = usage.facts_pruned ?? usage.logic_prunings ?? 0;

  if (loading) {
    return (
      <div className="pt-20 font-mono text-sm uppercase tracking-[0.18em] text-[var(--muted)]">
        Loading workspace...
      </div>
    );
  }

  return (
    <>
      {/* Page header */}
      <div className="dash-page-header">
        <div className="dash-page-header-row">
          <div>
            <p className="eyebrow">Workspace</p>
            <h1 className="dash-page-title mt-2">Overview</h1>
            <p className="dash-page-subtitle">Your projects, usage, and runtime state.</p>
          </div>
          <button onClick={() => setShowCreate(true)} className="button-solid">
            New Project
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="dash-stat-grid">
        <div className="dash-stat-card">
          <p className="field-label">Projects</p>
          <p className="metric-value mt-3">{sessions.length}</p>
        </div>
        <div className="dash-stat-card">
          <p className="field-label">Facts asserted</p>
          <p className="metric-value mt-3">{usage.facts_asserted || 0}</p>
        </div>
        <div className="dash-stat-card">
          <p className="field-label">Logic prunings</p>
          <p className="metric-value mt-3">{prunedCount}</p>
        </div>
        <div className="dash-stat-card">
          <p className="field-label">API requests</p>
          <p className="metric-value mt-3">{usage.api_requests || 0}</p>
        </div>
      </div>

      {/* Recent projects */}
      <div className="dash-section">
        <div className="dash-section-header">
          <h2 className="dash-section-title">Recent projects</h2>
          <a href="/dashboard/projects" className="text-link">
            View all
          </a>
        </div>

        {sessions.length === 0 ? (
          <div className="dash-empty">
            <p className="copy-tone font-copy text-lg leading-7">
              No projects yet. Create one to start asserting facts.
            </p>
            <button onClick={() => setShowCreate(true)} className="button-ghost">
              Create first project
            </button>
          </div>
        ) : (
          <div className="project-grid">
            {sessions.slice(0, 6).map((sess) => (
              <a
                key={sess.id}
                href={`/dashboard/session/${sess.id}`}
                className="project-card"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <h3 className="truncate text-lg tracking-[-0.03em]">
                      {sess.name || sess.id}
                    </h3>
                    {sess.name && (
                      <p className="mt-0.5 truncate font-mono text-[0.7rem] tracking-[0.08em] text-[var(--muted)]">
                        {sess.id}
                      </p>
                    )}
                    {sess.description && (
                      <p className="mt-2 font-copy text-sm leading-6 text-[var(--muted)] line-clamp-2">
                        {sess.description}
                      </p>
                    )}
                  </div>
                  <span
                    className="status-pill flex-shrink-0"
                    style={{ borderColor: "var(--accent)", color: "var(--accent)" }}
                  >
                    Active
                  </span>
                </div>
                <div className="flex items-center justify-between border-t border-[var(--line)] pt-3">
                  <span className="font-mono text-[0.72rem] uppercase tracking-[0.14em] text-[var(--muted)]">
                    {sess.fact_count || 0} facts
                  </span>
                  <span className="font-mono text-[0.72rem] tracking-[0.08em] text-[var(--muted)]">
                    {formatRelative(sess.last_activity_at || 0)}
                  </span>
                </div>
              </a>
            ))}
          </div>
        )}
      </div>

      {/* New project modal */}
      {showCreate && (
        <div className="modal-backdrop" onClick={() => setShowCreate(false)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <h2 className="mb-6 text-2xl tracking-[-0.04em]">New project</h2>
            <form onSubmit={handleCreate} className="space-y-5">
              <div className="flex flex-col gap-2">
                <label className="field-label">Project name</label>
                <input
                  autoFocus
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="my-approval-agent"
                  className="field"
                />
              </div>
              <div className="flex flex-col gap-2">
                <label className="field-label">Description (optional)</label>
                <input
                  type="text"
                  value={newDesc}
                  onChange={(e) => setNewDesc(e.target.value)}
                  placeholder="What this session tracks"
                  className="field"
                />
              </div>
              <div className="flex gap-3 pt-2">
                <button
                  type="submit"
                  disabled={creating}
                  className="button-solid flex-1 disabled:opacity-60"
                >
                  {creating ? "Creating..." : "Create project"}
                </button>
                <button
                  type="button"
                  onClick={() => setShowCreate(false)}
                  className="button-ghost"
                >
                  Cancel
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  );
}
