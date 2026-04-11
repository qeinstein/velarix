"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api";

type Invitation = {
  id: string;
  email: string;
  role: string;
  created_at: number;
  expires_at: number;
  accepted_at?: number;
  revoked_at?: number;
};

function formatDate(ms: number) {
  if (!ms) return "—";
  return new Date(ms).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function inviteStatus(inv: Invitation): { label: string; accent: boolean } {
  if (inv.revoked_at) return { label: "Revoked", accent: false };
  if (inv.accepted_at) return { label: "Accepted", accent: true };
  if (Date.now() > inv.expires_at) return { label: "Expired", accent: false };
  return { label: "Pending", accent: true };
}

export default function TeamPage() {
  const router = useRouter();
  const [members, setMembers] = useState<string[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [loading, setLoading] = useState(true);
  const [showInvite, setShowInvite] = useState(false);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("member");
  const [inviting, setInviting] = useState(false);
  const [inviteError, setInviteError] = useState("");
  const [inviteLink, setInviteLink] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<string | null>(null);

  const fetchTeam = async () => {
    try {
      const [membersRes, invRes] = await Promise.all([
        apiFetch("/v1/org/users"),
        apiFetch("/v1/org/invitations"),
      ]);
      if (membersRes.status === 401) { router.push("/login"); return; }
      if (membersRes.ok) {
        const data = await membersRes.json();
        setMembers(data?.items || []);
      }
      if (invRes.ok) {
        const data = await invRes.json();
        setInvitations(data?.items || []);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void fetchTeam(); }, []);

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    setInviteError("");
    setInviting(true);
    try {
      const res = await apiFetch("/v1/org/invitations", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: inviteEmail.trim().toLowerCase(), role: inviteRole }),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || "Failed to send invitation");
      }
      const data = await res.json();
      const token = data.invite_token;
      const link = `${window.location.origin}/invite?token=${token}`;
      setInviteLink(link);
      setInvitations((prev) => [data.invitation, ...prev]);
      setInviteEmail("");
    } catch (err) {
      setInviteError(err instanceof Error ? err.message : "Failed to invite");
    } finally {
      setInviting(false);
    }
  };

  const handleRevoke = async (invId: string) => {
    if (!confirm("Revoke this invitation?")) return;
    setRevoking(invId);
    try {
      const res = await apiFetch(`/v1/org/invitations/${invId}/revoke`, { method: "POST" });
      if (res.ok) {
        setInvitations((prev) =>
          prev.map((inv) =>
            inv.id === invId ? { ...inv, revoked_at: Date.now() } : inv
          )
        );
      }
    } finally {
      setRevoking(null);
    }
  };

  if (loading) {
    return (
      <div className="pt-20 font-mono text-sm uppercase tracking-[0.18em] text-[var(--muted)]">
        Loading team...
      </div>
    );
  }

  const pendingInvitations = invitations.filter(
    (inv) => !inv.revoked_at && !inv.accepted_at && Date.now() <= inv.expires_at
  );
  const pastInvitations = invitations.filter(
    (inv) => inv.revoked_at || inv.accepted_at || Date.now() > inv.expires_at
  );

  return (
    <>
      <div className="dash-page-header">
        <div className="dash-page-header-row">
          <div>
            <p className="eyebrow">Organisation</p>
            <h1 className="dash-page-title mt-2">Team</h1>
            <p className="dash-page-subtitle">
              Manage members and invite collaborators to your organisation.
            </p>
          </div>
          <button onClick={() => { setShowInvite(true); setInviteLink(null); setInviteError(""); }} className="button-solid">
            Invite member
          </button>
        </div>
      </div>

      {/* Members */}
      <div className="dash-section">
        <div className="dash-section-header">
          <h2 className="dash-section-title">Members</h2>
          <span className="status-pill">{members.length} total</span>
        </div>

        {members.length === 0 ? (
          <div className="dash-empty">
            <p className="copy-tone font-copy text-lg">No members found.</p>
          </div>
        ) : (
          <div className="overflow-hidden border border-[var(--line)]">
            <table className="dash-table">
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {members.map((email) => (
                  <tr key={email}>
                    <td>
                      <span className="font-mono text-sm">{email}</span>
                    </td>
                    <td>
                      <span
                        className="status-pill"
                        style={{ borderColor: "var(--accent)", color: "var(--accent)" }}
                      >
                        Active
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Pending invitations */}
      {pendingInvitations.length > 0 && (
        <div className="dash-section">
          <div className="dash-section-header">
            <h2 className="dash-section-title">Pending invitations</h2>
          </div>
          <div className="overflow-hidden border border-[var(--line)]">
            <table className="dash-table">
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Role</th>
                  <th>Expires</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {pendingInvitations.map((inv) => (
                  <tr key={inv.id}>
                    <td>
                      <span className="font-mono text-sm">{inv.email}</span>
                    </td>
                    <td>
                      <span className="status-pill">{inv.role}</span>
                    </td>
                    <td>
                      <span className="font-mono text-sm text-[var(--muted)]">
                        {formatDate(inv.expires_at)}
                      </span>
                    </td>
                    <td>
                      <button
                        onClick={() => handleRevoke(inv.id)}
                        disabled={revoking === inv.id}
                        className="font-mono text-[0.7rem] uppercase tracking-[0.14em] text-[var(--muted)] transition-colors hover:text-red-500 disabled:opacity-40"
                      >
                        {revoking === inv.id ? "..." : "Revoke"}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Past invitations */}
      {pastInvitations.length > 0 && (
        <div className="dash-section">
          <div className="dash-section-header">
            <h2 className="dash-section-title">Past invitations</h2>
          </div>
          <div className="overflow-hidden border border-[var(--line)]">
            <table className="dash-table">
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Role</th>
                  <th>Sent</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {pastInvitations.map((inv) => {
                  const st = inviteStatus(inv);
                  return (
                    <tr key={inv.id}>
                      <td>
                        <span className="font-mono text-sm text-[var(--muted)]">{inv.email}</span>
                      </td>
                      <td>
                        <span className="status-pill">{inv.role}</span>
                      </td>
                      <td>
                        <span className="font-mono text-sm text-[var(--muted)]">
                          {formatDate(inv.created_at)}
                        </span>
                      </td>
                      <td>
                        <span
                          className="status-pill"
                          style={
                            st.accent
                              ? { borderColor: "var(--accent)", color: "var(--accent)" }
                              : {}
                          }
                        >
                          {st.label}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Invite modal */}
      {showInvite && (
        <div className="modal-backdrop" onClick={() => setShowInvite(false)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <h2 className="mb-6 text-2xl tracking-[-0.04em]">Invite team member</h2>

            {inviteLink ? (
              <div className="space-y-5">
                <p className="copy-tone font-copy text-base leading-7">
                  Invitation created. Share this link with your team member — it expires in 7 days.
                </p>
                <div className="dark-surface code-panel p-4">
                  <pre className="break-all text-[0.78rem]">{inviteLink}</pre>
                </div>
                <div className="flex gap-3">
                  <button
                    onClick={() => navigator.clipboard.writeText(inviteLink)}
                    className="button-solid flex-1"
                  >
                    Copy link
                  </button>
                  <button onClick={() => setShowInvite(false)} className="button-ghost">
                    Close
                  </button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleInvite} className="space-y-5">
                {inviteError && <div className="error-box">{inviteError}</div>}
                <div className="flex flex-col gap-2">
                  <label className="field-label">Email address</label>
                  <input
                    autoFocus
                    type="email"
                    value={inviteEmail}
                    onChange={(e) => setInviteEmail(e.target.value)}
                    placeholder="colleague@company.com"
                    className="field"
                    required
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <label className="field-label">Role</label>
                  <select
                    value={inviteRole}
                    onChange={(e) => setInviteRole(e.target.value)}
                    className="field"
                  >
                    <option value="member">Member</option>
                    <option value="admin">Admin</option>
                    <option value="auditor">Auditor (read-only)</option>
                  </select>
                </div>
                <div className="flex gap-3 pt-2">
                  <button
                    type="submit"
                    disabled={inviting}
                    className="button-solid flex-1 disabled:opacity-60"
                  >
                    {inviting ? "Sending..." : "Send invitation"}
                  </button>
                  <button type="button" onClick={() => setShowInvite(false)} className="button-ghost">
                    Cancel
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </>
  );
}
