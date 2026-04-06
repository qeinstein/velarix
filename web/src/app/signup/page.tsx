"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

export default function Signup() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSignup = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const res = await fetch(`${process.env.NEXT_PUBLIC_VELARIX_API_URL || "http://localhost:8080"}/v1/auth/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });

      if (!res.ok) {
        throw new Error("Failed to create account");
      }

      const loginRes = await fetch(`${process.env.NEXT_PUBLIC_VELARIX_API_URL || "http://localhost:8080"}/v1/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });

      if (!loginRes.ok) {
        throw new Error("Account created, but failed to log in automatically");
      }

      const data = await loginRes.json();
      localStorage.setItem("vlx_token", data.token);
      router.push("/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to create account");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="max-w-xl pb-24 pt-14 md:pt-20">
      <div className="space-y-6">
        <p className="eyebrow">Account</p>
        <h1 className="font-display text-[clamp(3rem,8vw,5rem)] leading-[0.92] tracking-[-0.07em]">
          Create your workspace.
        </h1>
        <p className="copy-tone font-copy text-xl leading-8">
          Start with Cloud if you want managed sessions, or use this account first and decide
          later.
        </p>
      </div>

      <form onSubmit={handleSignup} className="section-rule mt-10 space-y-6">
        {error && <div className="error-box">{error}</div>}
        <div className="flex flex-col gap-2">
          <label className="field-label">Work email</label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="alan@turing.com"
            className="field"
            required
          />
        </div>
        <div className="flex flex-col gap-2">
          <label className="field-label">Password</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="••••••••"
            className="field"
            required
          />
        </div>

        <button disabled={loading} type="submit" className="button-solid mt-2 w-full disabled:opacity-60">
          {loading ? "Creating..." : "Create Account"}
        </button>
      </form>

      <div className="mt-8 flex flex-wrap gap-x-6 gap-y-3">
        <a href="/login" className="text-link">
          Sign in
        </a>
        <a href="/docs" className="text-link">
          Read docs
        </a>
      </div>
    </main>
  );
}
