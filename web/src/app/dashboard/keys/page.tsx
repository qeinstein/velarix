"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api";

type APIKey = {
  id?: string;
  key?: string;
  key_prefix?: string;
  key_last4?: string;
  label: string;
  scopes?: string[];
  created_at?: number;
  last_used_at?: number;
  expires_at?: number;
  is_revoked?: boolean;
};

function formatDate(ms: number) {
  if (!ms || ms === 9999999999999) return "—";
  return new Date(ms).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

const SCOPE_OPTIONS = ["read", "write", "export"];

export default function KeysPage() {
  const router = useRouter();
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showGenerate, setShowGenerate] = useState(false);
  const [label, setLabel] = useState("");
  const [scopes, setScopes] = useState<string[]>(["read", "write"]);
  const [generating, setGenerating] = useState(false);
  const [newKey, setNewKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState<string | null>(null);

  const fetchKeys = async () => {
    try {
      const res = await apiFetch("/v1/keys");
      if (res.status === 401) { router.push("/login"); return; }
      if (res.ok) {
        const data = await res.json();
        setKeys(Array.isArray(data) ? data : []);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void fetchKeys(); }, []);

  const handleGenerate = async (e: React.FormEvent) => {
    e.preventDefault();
    setGenerating(true);
    try {
      const res = await apiFetch("/v1/keys/generate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ label: label.trim() || "New key", scopes }),
      });
      if (res.ok) {
        const created = await res.json();
        setNewKey(created.key || null);
        setKeys((prev) => [...prev, created]);
        setLabel("");
      }
    } finally {
      setGenerating(false);
    }
  };

  const handleRevoke = async (keyId: string) => {
    if (!confirm("Revoke this API key? This cannot be undone.")) return;
    setRevoking(keyId);
    try {
      const res = await apiFetch(`/v1/keys/${keyId}`, { method: "DELETE" });
      if (res.ok) {
        setKeys((prev) =>
          prev.map((k) => (k.id === keyId ? { ...k, is_revoked: true } : k))
        );
      }
    } finally {
      setRevoking(null);
    }
  };

  const handleCopy = (val: string) => {
    navigator.clipboard.writeText(val);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const toggleScope = (scope: string) => {
    setScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
    );
  };

  const activeKeys = keys.filter((k) => !k.is_revoked);
  const revokedKeys = keys.filter((k) => k.is_revoked);

  if (loading) {
    return (
      <div className="pt-20 font-mono text-sm uppercase tracking-[0.18em] text-[var(--muted)]">
        Loading keys...
      </div>
    );
  }

  return (
    <>
      <div className="dash-page-header">
        <div className="dash-page-header-row">
          <div>
            <p className="eyebrow">Authentication</p>
            <h1 className="dash-page-title mt-2">API Keys</h1>
            <p className="dash-page-subtitle">
              Use these as <code className="font-mono text-base">Authorization: Bearer &lt;key&gt;</code> when
              calling the API programmatically.
            </p>
          </div>
          <button onClick={() => { setShowGenerate(true); setNewKey(null); }} className="button-solid">
            Generate key
          </button>
        </div>
      </div>

      {/* New key banner */}
      {newKey && (
        <div className="mb-8 border border-[var(--accent)] bg-[var(--accent-soft)] p-5">
          <p className="eyebrow mb-3" style={{ color: "var(--accent)" }}>
            Key generated — copy it now
          </p>
          <p className="mb-3 font-copy text-base leading-7 text-[var(--copy)]">
            This is the only time this key will be shown. Store it somewhere safe.
          </p>
          <div className="flex flex-col gap-3 md:flex-row md:items-stretch">
            <div className="dark-surface code-panel flex-1 p-4">
              <pre className="break-all text-[0.85rem]">{newKey}</pre>
            </div>
            <button onClick={() => handleCopy(newKey)} className="button-solid whitespace-nowrap">
              {copied ? "Copied!" : "Copy key"}
            </button>
          </div>
        </div>
      )}

      {/* Active keys */}
      <div className="dash-section">
        <div className="dash-section-header">
          <h2 className="dash-section-title">Active keys</h2>
          <span className="status-pill">{activeKeys.length}</span>
        </div>

        {activeKeys.length === 0 ? (
          <div className="dash-empty">
            <p className="copy-tone font-copy text-lg">
              No active keys. Generate one to start making API requests.
            </p>
            <button onClick={() => { setShowGenerate(true); setNewKey(null); }} className="button-ghost">
              Generate first key
            </button>
          </div>
        ) : (
          <div className="overflow-hidden border border-[var(--line)]">
            <table className="dash-table">
              <thead>
                <tr>
                  <th>Label</th>
                  <th>Key</th>
                  <th>Scopes</th>
                  <th>Created</th>
                  <th>Last used</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {activeKeys.map((k) => {
                  const displayId = k.id || k.key_prefix || "—";
                  const displayKey = k.key_prefix && k.key_last4
                    ? `${k.key_prefix}...${k.key_last4}`
                    : k.key_prefix || displayId;
                  return (
                    <tr key={displayId}>
                      <td>
                        <span className="font-medium tracking-[-0.02em]">{k.label}</span>
                      </td>
                      <td>
                        <code className="font-mono text-[0.78rem] text-[var(--muted)]">
                          {displayKey}
                        </code>
                      </td>
                      <td>
                        <div className="flex flex-wrap gap-1">
                          {(k.scopes || []).map((s) => (
                            <span key={s} className="status-pill text-[0.65rem]">{s}</span>
                          ))}
                        </div>
                      </td>
                      <td>
                        <span className="font-mono text-sm text-[var(--muted)]">
                          {formatDate(k.created_at || 0)}
                        </span>
                      </td>
                      <td>
                        <span className="font-mono text-sm text-[var(--muted)]">
                          {formatDate(k.last_used_at || 0)}
                        </span>
                      </td>
                      <td>
                        <button
                          onClick={() => handleRevoke(displayId)}
                          disabled={revoking === displayId}
                          className="font-mono text-[0.7rem] uppercase tracking-[0.14em] text-[var(--muted)] transition-colors hover:text-red-500 disabled:opacity-40"
                        >
                          {revoking === displayId ? "..." : "Revoke"}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Revoked keys (collapsed) */}
      {revokedKeys.length > 0 && (
        <div className="dash-section">
          <details>
            <summary className="cursor-pointer">
              <span className="dash-section-title">Revoked keys ({revokedKeys.length})</span>
            </summary>
            <div className="mt-4 overflow-hidden border border-[var(--line)] opacity-60">
              <table className="dash-table">
                <thead>
                  <tr>
                    <th>Label</th>
                    <th>Key</th>
                    <th>Created</th>
                  </tr>
                </thead>
                <tbody>
                  {revokedKeys.map((k) => {
                    const displayId = k.id || k.key_prefix || "—";
                    return (
                      <tr key={displayId}>
                        <td>
                          <span className="line-through opacity-60">{k.label}</span>
                        </td>
                        <td>
                          <code className="font-mono text-[0.78rem] text-[var(--muted)] line-through">
                            {k.key_prefix || displayId}
                          </code>
                        </td>
                        <td>
                          <span className="font-mono text-sm text-[var(--muted)]">
                            {formatDate(k.created_at || 0)}
                          </span>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </details>
        </div>
      )}

      {/* Generate modal */}
      {showGenerate && !newKey && (
        <div className="modal-backdrop" onClick={() => setShowGenerate(false)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <h2 className="mb-6 text-2xl tracking-[-0.04em]">Generate API key</h2>
            <form onSubmit={handleGenerate} className="space-y-5">
              <div className="flex flex-col gap-2">
                <label className="field-label">Label</label>
                <input
                  autoFocus
                  type="text"
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                  placeholder="Production, CI, Development…"
                  className="field"
                />
              </div>
              <div className="flex flex-col gap-3">
                <label className="field-label">Scopes</label>
                {SCOPE_OPTIONS.map((scope) => (
                  <label key={scope} className="flex cursor-pointer items-center gap-3">
                    <input
                      type="checkbox"
                      checked={scopes.includes(scope)}
                      onChange={() => toggleScope(scope)}
                      className="h-4 w-4 cursor-pointer"
                    />
                    <span className="font-mono text-sm uppercase tracking-[0.12em]">{scope}</span>
                  </label>
                ))}
              </div>
              <div className="flex gap-3 pt-2">
                <button
                  type="submit"
                  disabled={generating || scopes.length === 0}
                  className="button-solid flex-1 disabled:opacity-60"
                >
                  {generating ? "Generating..." : "Generate key"}
                </button>
                <button type="button" onClick={() => setShowGenerate(false)} className="button-ghost">
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
