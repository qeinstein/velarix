"use client";

import { useRouter } from "next/navigation";

export default function CheckoutCancelPage() {
  const router = useRouter();

  return (
    <div className="flex min-h-screen items-center justify-center p-8 font-mono">
      <div className="max-w-md text-center space-y-6">
        <div className="text-5xl">✕</div>
        <h1 className="text-2xl font-semibold tracking-tight">Checkout cancelled</h1>
        <p className="text-[var(--muted)] text-sm leading-relaxed">
          No payment was taken. You can return to settings and try again whenever you are ready.
        </p>
        <button
          onClick={() => router.push("/dashboard/settings")}
          className="button-solid"
        >
          Back to settings
        </button>
      </div>
    </div>
  );
}
