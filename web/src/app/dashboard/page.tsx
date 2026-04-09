"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "../../lib/api";

type ApiKey = {
  id?: string;
  key?: string;
};

type Session = {
  id: string;
  fact_count?: number;
};

type Usage = {
  api_requests?: number;
  facts_asserted?: number;
  facts_pruned?: number;
  logic_prunings?: number;
};

type Billing = {
  billing_email?: string;
  plan?: string;
  status?: string;
} | null;

export default function Dashboard() {
  const router = useRouter();
  const [copied, setCopied] = useState(false);
  const [apiKeys, setApiKeys] = useState<ApiKey[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [usage, setUsage] = useState<Usage>({ api_requests: 0, facts_asserted: 0, logic_prunings: 0 });
  const [billing, setBilling] = useState<Billing>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const keysRes = await apiFetch("/v1/keys");
        if (keysRes.status === 401) {
          router.push("/login");
          return;
        }
        if (keysRes.ok) {
          const keysData = await keysRes.json();
          setApiKeys(keysData || []);
        }

        const usageRes = await apiFetch("/v1/org/usage");
        if (usageRes.ok) {
          const usageData = await usageRes.json();
          setUsage(usageData || { api_requests: 0, facts_asserted: 0, logic_prunings: 0 });
        }

        const sessionsRes = await apiFetch("/v1/org/sessions");
        if (sessionsRes.ok) {
          const sessionsData = await sessionsRes.json();
          setSessions(sessionsData?.items || []);
        }

        const billingRes = await apiFetch("/v1/org/billing");
        if (billingRes.ok) {
          const billingData = await billingRes.json();
          setBilling(billingData);
        }
      } catch (err) {
        console.error(err);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [router]);

  const generateKey = async () => {
    const res = await apiFetch("/v1/keys/generate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ label: "Production", scopes: ["read", "write"] }),
    });
    if (res.status === 401) {
      router.push("/login");
      return;
    }
    if (res.ok) {
      const newKey = await res.json();
      setApiKeys((current) => [...current, newKey]);
    }
  };

  const handleCopy = (key: string) => {
    navigator.clipboard.writeText(key);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleSignOut = () => {
    void apiFetch("/v1/auth/logout", { method: "POST" });
    localStorage.removeItem("vlx_token");
    router.push("/");
  };

  if (loading) {
    return (
      <main className="pt-20 font-mono text-sm uppercase tracking-[0.18em] text-[var(--muted)]">
        Loading workspace...
      </main>
    );
  }

  const latestKey = apiKeys[apiKeys.length - 1];
  const prunedCount = usage.facts_pruned ?? usage.logic_prunings ?? 0;

  return (
    <main className="pb-24 pt-8">
      <header className="flex flex-col gap-5 border-b border-[var(--line)] pb-6 md:flex-row md:items-end md:justify-between">
        <div className="space-y-2">
          <p className="eyebrow">Workspace</p>
          <h1 className="font-display text-[clamp(3rem,8vw,5rem)] leading-[0.92] tracking-[-0.07em]">
            Dashboard
          </h1>
          <p className="copy-tone font-copy text-lg leading-7">
            Usage, billing, keys and active sessions.
          </p>
        </div>

        <button onClick={handleSignOut} className="text-link w-fit">
          Sign Out
        </button>
      </header>

      <section className="section-rule mt-12 grid gap-6 lg:grid-cols-[11rem_1fr]">
        <p className="eyebrow">Billing</p>
        <div className="surface space-y-6 p-6 md:p-8">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="space-y-2">
              <h2 className="text-2xl tracking-[-0.05em]">Plan and billing</h2>
              <p className="copy-tone font-copy text-lg leading-7">
                Manage your Velarix Cloud subscription and billing contact.
              </p>
            </div>
            <span className="status-pill">{billing?.plan || "Free"} plan</span>
          </div>

          <div className="grid gap-6 border-t border-[var(--line)] pt-6 md:grid-cols-2">
            <div className="space-y-2">
              <p className="field-label">Billing email</p>
              <p className="font-mono text-sm">{billing?.billing_email || "Not set"}</p>
            </div>
            <div className="space-y-2">
              <p className="field-label">Status</p>
              <p className="font-mono text-sm">{billing?.status || "Active"}</p>
            </div>
          </div>

          <button className="button-ghost w-fit">Manage subscription</button>
        </div>
      </section>

      <section className="section-rule mt-12 grid gap-6 lg:grid-cols-[11rem_1fr]">
        <div className="space-y-2">
          <p className="eyebrow">API key</p>
          <p className="copy-tone font-copy text-lg leading-7">
            Use this as <code className="font-mono text-base">VELARIX_API_KEY</code> in your
            environment.
          </p>
        </div>

        {apiKeys.length === 0 ? (
          <button onClick={generateKey} className="button-solid w-fit">
            Generate New Key
          </button>
        ) : (
          <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-stretch">
            <div className="dark-surface code-panel p-4 md:p-5">
              <pre>{latestKey?.key || latestKey?.id}</pre>
            </div>
            <button 
              onClick={() => handleCopy(latestKey?.key || latestKey?.id || "")}
              className="button-solid whitespace-nowrap"
            >
              {copied ? "Copied!" : "Copy Key"}
            </button>
          </div>
        )}
      </section>

      <section className="section-rule mt-12 grid gap-6 lg:grid-cols-[11rem_1fr]">
        <div className="space-y-2">
          <p className="eyebrow">Usage</p>
          <p className="copy-tone font-copy text-lg leading-7">
            Metrics for the current billing period.
          </p>
        </div>

        <div className="grid gap-4 md:grid-cols-3">
          <div className="surface p-6">
            <p className="field-label">Facts asserted</p>
            <p className="metric-value mt-4">{usage.facts_asserted || 0}</p>
          </div>
          <div className="surface p-6">
            <p className="field-label">Logic prunings</p>
            <p className="metric-value mt-4">{prunedCount}</p>
          </div>
          <div className="surface p-6">
            <p className="field-label">API requests</p>
            <p className="metric-value mt-4">{usage.api_requests || 0}</p>
          </div>
        </div>
      </section>

      <section className="section-rule mt-12 grid gap-6 lg:grid-cols-[11rem_1fr]">
        <div className="space-y-4">
          <p className="eyebrow">Sessions</p>
          <a href="/docs" className="text-link w-fit">
            SDK docs
          </a>
        </div>
        <div className="overflow-hidden border border-[var(--line)]">
          <div className="grid grid-cols-3 gap-4 bg-[var(--panel-soft)] px-4 py-3 table-header">
            <div>Session ID</div>
            <div>Fact Count</div>
            <div>Status</div>
          </div>
          {sessions.length === 0 ? (
            <div className="px-4 py-6 font-copy text-lg leading-7 text-[var(--muted)]">
              No active sessions. Connect your SDK to start asserting facts.
            </div>
          ) : (
            sessions.map((session) => (
              <a
                key={session.id}
                href={`/dashboard/session/${session.id}`}
                className="grid grid-cols-3 gap-4 border-t border-[var(--line)] px-4 py-4 text-sm transition-colors hover:bg-[var(--panel-hover)]"
              >
                <div className="font-mono truncate pr-4">{session.id}</div>
                <div>{session.fact_count || 0}</div>
                <div className="font-mono uppercase tracking-[0.16em]" style={{ color: "var(--accent)" }}>
                  Active
                </div>
              </a>
            ))
          )}
        </div>
      </section>
    </main>
  );
}
