"use client";

import { useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { apiFetch } from "@/lib/api";

function InviteForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";

  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleAccept = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    if (password !== confirm) {
      setError("Passwords do not match");
      return;
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    if (!token) {
      setError("Invalid invitation link");
      return;
    }

    setLoading(true);
    try {
      const res = await apiFetch("/v1/invitations/accept", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, password }),
      });

      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || "Failed to accept invitation");
      }

      router.push("/login?invited=1");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to accept invitation");
    } finally {
      setLoading(false);
    }
  };

  if (!token) {
    return (
      <div className="error-box mt-10">
        Invalid or missing invitation token. Please use the link from your invitation email.
      </div>
    );
  }

  return (
    <form onSubmit={handleAccept} className="section-rule mt-10 space-y-6">
      {error && <div className="error-box">{error}</div>}
      <div className="flex flex-col gap-2">
        <label className="field-label">Set your password</label>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="••••••••"
          className="field"
          required
          minLength={8}
        />
      </div>
      <div className="flex flex-col gap-2">
        <label className="field-label">Confirm password</label>
        <input
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          placeholder="••••••••"
          className="field"
          required
          minLength={8}
        />
      </div>
      <button
        disabled={loading}
        type="submit"
        className="button-solid mt-2 w-full disabled:opacity-60"
      >
        {loading ? "Joining..." : "Accept and Join"}
      </button>
    </form>
  );
}

export default function InvitePage() {
  return (
    <main className="max-w-xl pb-24 pt-14 md:pt-20">
      <div className="space-y-6">
        <p className="eyebrow">Invitation</p>
        <h1 className="font-display text-[clamp(3rem,8vw,5rem)] leading-[0.92] tracking-[-0.07em]">
          Join your team.
        </h1>
        <p className="copy-tone font-copy text-xl leading-8">
          Set a password to activate your account and join the organisation that invited you.
        </p>
      </div>
      <Suspense
        fallback={
          <div className="pt-10 font-mono text-sm uppercase tracking-[0.18em] text-[var(--muted)]">
            Loading...
          </div>
        }
      >
        <InviteForm />
      </Suspense>
      <div className="mt-8 flex flex-wrap gap-x-6 gap-y-3">
        <a href="/login" className="text-link">
          Already have an account?
        </a>
      </div>
    </main>
  );
}
