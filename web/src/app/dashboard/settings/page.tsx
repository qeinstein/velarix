"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api";

type Org = {
  id: string;
  name: string;
};

type BillingInfo = {
  plan?: string;
  status?: string;
  billing_email?: string;
  current_period_end?: number;
};

export default function SettingsPage() {
  const router = useRouter();
  const [org, setOrg] = useState<Org | null>(null);
  const [billing, setBilling] = useState<BillingInfo | null>(null);
  const [orgName, setOrgName] = useState("");
  const [savingOrg, setSavingOrg] = useState(false);
  const [orgSaved, setOrgSaved] = useState(false);
  const [orgError, setOrgError] = useState("");

  const [currentPw, setCurrentPw] = useState("");
  const [newPw, setNewPw] = useState("");
  const [confirmPw, setConfirmPw] = useState("");
  const [savingPw, setSavingPw] = useState(false);
  const [pwSaved, setPwSaved] = useState(false);
  const [pwError, setPwError] = useState("");

  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [orgRes, billingRes] = await Promise.all([
          apiFetch("/v1/org"),
          apiFetch("/v1/billing/subscription"),
        ]);
        if (orgRes.status === 401) { router.push("/login"); return; }
        if (orgRes.ok) {
          const data = await orgRes.json();
          setOrg(data);
          setOrgName(data.name || "");
        }
        if (billingRes.ok) {
          setBilling(await billingRes.json());
        }
      } finally {
        setLoading(false);
      }
    };
    void fetchData();
  }, [router]);

  const handleSaveOrg = async (e: React.FormEvent) => {
    e.preventDefault();
    setOrgError("");
    setSavingOrg(true);
    try {
      const res = await apiFetch("/v1/org", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: orgName.trim() }),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || "Failed to update");
      }
      setOrgSaved(true);
      setTimeout(() => setOrgSaved(false), 3000);
    } catch (err) {
      setOrgError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSavingOrg(false);
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPwError("");
    if (newPw !== confirmPw) { setPwError("Passwords do not match"); return; }
    if (newPw.length < 8) { setPwError("New password must be at least 8 characters"); return; }
    setSavingPw(true);
    try {
      const res = await apiFetch("/v1/me/change-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ current_password: currentPw, new_password: newPw }),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || "Failed to change password");
      }
      setCurrentPw("");
      setNewPw("");
      setConfirmPw("");
      setPwSaved(true);
      setTimeout(() => setPwSaved(false), 3000);
    } catch (err) {
      setPwError(err instanceof Error ? err.message : "Failed to change password");
    } finally {
      setSavingPw(false);
    }
  };

  if (loading) {
    return (
      <div className="pt-20 font-mono text-sm uppercase tracking-[0.18em] text-[var(--muted)]">
        Loading settings...
      </div>
    );
  }

  return (
    <>
      <div className="dash-page-header">
        <div>
          <p className="eyebrow">Configuration</p>
          <h1 className="dash-page-title mt-2">Settings</h1>
          <p className="dash-page-subtitle">Organisation, billing, and account preferences.</p>
        </div>
      </div>

      {/* Organisation */}
      <div className="dash-section">
        <div className="dash-section-header">
          <h2 className="dash-section-title">Organisation</h2>
        </div>
        <div className="surface p-6 md:p-8">
          {org && (
            <div className="mb-6 flex flex-col gap-1">
              <p className="field-label">Organisation ID</p>
              <code className="font-mono text-sm text-[var(--muted)]">{org.id}</code>
            </div>
          )}
          <form onSubmit={handleSaveOrg} className="max-w-md space-y-5">
            {orgError && <div className="error-box">{orgError}</div>}
            <div className="flex flex-col gap-2">
              <label className="field-label">Display name</label>
              <input
                type="text"
                value={orgName}
                onChange={(e) => setOrgName(e.target.value)}
                placeholder="My Organisation"
                className="field"
              />
            </div>
            <button
              type="submit"
              disabled={savingOrg}
              className="button-solid disabled:opacity-60"
            >
              {savingOrg ? "Saving..." : orgSaved ? "Saved!" : "Save changes"}
            </button>
          </form>
        </div>
      </div>

      {/* Billing */}
      <div className="dash-section">
        <div className="dash-section-header">
          <h2 className="dash-section-title">Billing</h2>
        </div>
        <div className="surface p-6 md:p-8">
          <div className="grid gap-6 md:grid-cols-3">
            <div className="space-y-2">
              <p className="field-label">Plan</p>
              <p className="text-lg font-medium capitalize">{billing?.plan || "Free"}</p>
            </div>
            <div className="space-y-2">
              <p className="field-label">Status</p>
              <p className="text-lg font-medium capitalize">{billing?.status || "Active"}</p>
            </div>
            <div className="space-y-2">
              <p className="field-label">Billing email</p>
              <p className="font-mono text-sm">{billing?.billing_email || "—"}</p>
            </div>
          </div>
          <div className="mt-6 border-t border-[var(--line)] pt-5">
            <a href="mailto:hello@velarix.com" className="button-ghost inline-flex">
              Manage billing
            </a>
          </div>
        </div>
      </div>

      {/* Change password */}
      <div className="dash-section">
        <div className="dash-section-header">
          <h2 className="dash-section-title">Change password</h2>
        </div>
        <div className="surface p-6 md:p-8">
          <form onSubmit={handleChangePassword} className="max-w-md space-y-5">
            {pwError && <div className="error-box">{pwError}</div>}
            <div className="flex flex-col gap-2">
              <label className="field-label">Current password</label>
              <input
                type="password"
                value={currentPw}
                onChange={(e) => setCurrentPw(e.target.value)}
                placeholder="••••••••"
                className="field"
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <label className="field-label">New password</label>
              <input
                type="password"
                value={newPw}
                onChange={(e) => setNewPw(e.target.value)}
                placeholder="••••••••"
                className="field"
                required
                minLength={8}
              />
            </div>
            <div className="flex flex-col gap-2">
              <label className="field-label">Confirm new password</label>
              <input
                type="password"
                value={confirmPw}
                onChange={(e) => setConfirmPw(e.target.value)}
                placeholder="••••••••"
                className="field"
                required
                minLength={8}
              />
            </div>
            <button
              type="submit"
              disabled={savingPw}
              className="button-solid disabled:opacity-60"
            >
              {savingPw ? "Updating..." : pwSaved ? "Password updated!" : "Change password"}
            </button>
          </form>
        </div>
      </div>
    </>
  );
}
