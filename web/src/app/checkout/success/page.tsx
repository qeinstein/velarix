"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

export default function CheckoutSuccessPage() {
  const router = useRouter();
  const [countdown, setCountdown] = useState(5);

  useEffect(() => {
    const interval = setInterval(() => {
      setCountdown((c) => {
        if (c <= 1) {
          clearInterval(interval);
          router.push("/dashboard/settings");
          return 0;
        }
        return c - 1;
      });
    }, 1000);
    return () => clearInterval(interval);
  }, [router]);

  return (
    <div className="flex min-h-screen items-center justify-center p-8 font-mono">
      <div className="max-w-md text-center space-y-6">
        <div className="text-5xl">✓</div>
        <h1 className="text-2xl font-semibold tracking-tight">Payment confirmed</h1>
        <p className="text-[var(--muted)] text-sm leading-relaxed">
          Your subscription is being activated. Plan changes take effect within a few seconds once
          Stripe confirms the payment.
        </p>
        <p className="text-xs text-[var(--muted)]">
          Redirecting to settings in {countdown}s…
        </p>
        <button
          onClick={() => router.push("/dashboard/settings")}
          className="button-solid"
        >
          Go to settings
        </button>
      </div>
    </div>
  );
}
