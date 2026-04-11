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

function formatDate(ms: number) {
  if (!ms) return "—";
  return new Date(ms).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function formatRelative(ms: number) {
  if (!ms) return "—";
  const diff = Date.now() - ms;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

export default function ProjectsPage() {
  const router = useRouter();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const fetchSessions = async () => {
    try {
      const res = await apiFetch("/v1/org/sessions?limit=200");
      if (res.status === 401) { router.push("/login"); return; }
      if (res.ok) {
        const data = await res.json();
        setSessions(data?.items || []);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void fetchSessions(); }, []);

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

  const handleDelete = async (sessionId: string) => {
    if (!confirm("Archive this project? It will no longer appear in your dashboard.")) return;
    setDeletingId(sessionId);
    try {
      const res = await apiFetch(`/v1/org/sessions/${sessionId}`, { method: "DELETE" });
      if (res.ok) {
        setSessions((prev) => prev.filter((s) => s.id !== sessionId));
      }
    } finally {
      setDeletingId(null);
    }
  };

  if (loading) {
    return (
      <div className="pt-20 font-mono text-sm uppercase tracking-[0.18em] text-[var(--muted)]">
        Loading projects...
      </div>
    );
  }

  return (
    <>
      <div className="dash-page-header">
        <div className="dash-page-header-row">
          <div>
            <p className="eyebrow">Workspace</p>
            <h1 className="dash-page-title mt-2">Projects</h1>
            <p className="dash-page-subtitle">
              Each project is a session — a scoped fact graph your agents write into.
            </p>
          </div>
          <button onClick={() => setShowCreate(true)} className="button-solid">
            New Project
          </button>
        </div>
      </div>

      {sessions.length === 0 ? (
        <div className="dash-empty">
          <p className="copy-tone font-copy text-lg leading-7">
            No projects yet. Connect your SDK to a session id, or create one here first.
          </p>
          <button onClick={() => setShowCreate(true)} className="button-ghost">
            Create first project
          </button>
        </div>
      ) : (
        <div className="overflow-hidden border border-[var(--line)]">
          <table className="dash-table w-full">
            <thead>
              <tr>
                <th>Name</th>
                <th>Session ID</th>
                <th>Facts</th>
                <th>Last active</th>
                <th>Created</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {sessions.map((sess) => (
                <tr
                  key={sess.id}
                  className="cursor-pointer"
                  onClick={() => router.push(`/dashboard/session/${sess.id}`)}
                >
                  <td>
                    <div className="flex flex-col gap-0.5">
                      <span className="font-medium tracking-[-0.02em]">
                        {sess.name || <span className="text-[var(--muted)]">Unnamed</span>}
                      </span>
                      {sess.description && (
                        <span className="font-copy text-sm text-[var(--muted)]">
                          {sess.description}
                        </span>
                      )}
                    </div>
                  </td>
                  <td>
                    <code className="font-mono text-[0.78rem] text-[var(--muted)]">{sess.id}</code>
                  </td>
                  <td>
                    <span className="font-mono text-sm">{sess.fact_count || 0}</span>
                  </td>
                  <td>
                    <span className="font-mono text-sm text-[var(--muted)]">
                      {formatRelative(sess.last_activity_at || 0)}
                    </span>
                  </td>
                  <td>
                    <span className="font-mono text-sm text-[var(--muted)]">
                      {formatDate(sess.created_at || 0)}
                    </span>
                  </td>
                  <td onClick={(e) => e.stopPropagation()}>
                    <button
                      onClick={() => handleDelete(sess.id)}
                      disabled={deletingId === sess.id}
                      className="font-mono text-[0.7rem] uppercase tracking-[0.14em] text-[var(--muted)] transition-colors hover:text-red-500 disabled:opacity-40"
                    >
                      {deletingId === sess.id ? "..." : "Archive"}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

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
                <button type="button" onClick={() => setShowCreate(false)} className="button-ghost">
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
