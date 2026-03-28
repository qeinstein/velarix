import type { Metadata } from "next";
import type { ReactNode } from "react";

import { DocsRouteTransition } from "../../components/docs/docs-route-transition";

export const metadata: Metadata = {
  title: "Velarix Docs",
  description:
    "Technical documentation for Velarix: quickstart, causal logic, invalidation, security, and API surfaces.",
};

export default function DocsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="docs-sans min-h-screen bg-[#0A0A0C] text-white">
      <DocsRouteTransition>{children}</DocsRouteTransition>
    </div>
  );
}
