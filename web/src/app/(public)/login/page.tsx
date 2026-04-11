"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api";

export default function Login() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const res = await apiFetch("/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });

      if (!res.ok) {
        throw new Error("Invalid credentials");
      }

      localStorage.removeItem("vlx_token");
      router.push("/dashboard");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to sign in");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="max-w-xl pb-24 pt-14 md:pt-20">
      <div className="space-y-6">
        <p className="eyebrow">Login</p>
        <h1 className="font-display text-[clamp(3rem,8vw,5rem)] leading-[0.92] tracking-[-0.07em]">
          Access your workspace.
        </h1>
        <p className="copy-tone font-copy text-xl leading-8">
          Use the email that owns your Velarix account and API keys.
        </p>
      </div>

      <form onSubmit={handleLogin} className="section-rule mt-10 space-y-6">
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

        <button
          disabled={loading}
          type="submit"
          className="button-solid mt-2 w-full disabled:opacity-60"
        >
          {loading ? "Signing In..." : "Sign In"}
        </button>
      </form>

      <div className="mt-8 flex flex-wrap gap-x-6 gap-y-3">
        <a href="/signup" className="text-link">
          Create an account
        </a>
        <a href="/docs" className="text-link">
          Read docs
        </a>
      </div>
    </main>
  );
}
