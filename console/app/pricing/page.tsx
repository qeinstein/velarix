import type { ReactNode } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { Check } from "lucide-react";
import { Geist, Instrument_Serif } from "next/font/google";

const geist = Geist({
  subsets: ["latin"],
  display: "swap",
});

const instrumentSerif = Instrument_Serif({
  subsets: ["latin"],
  display: "swap",
  weight: "400",
  style: ["normal", "italic"],
});

type BillingCycle = "monthly" | "yearly";

type PricingPageProps = {
  searchParams?: Promise<{
    billing?: string | string[];
  }>;
};

type Tier = {
  name: string;
  purpose: string;
  description: string;
  featured?: boolean;
  monthly: {
    amount: string;
    cadence: string;
    note: string;
  };
  yearly: {
    amount: string;
    cadence: string;
    note: string;
  };
  features: Array<{
    label: string;
    italic?: boolean;
  }>;
};

type ComparisonCell = {
  kind: "text" | "yes" | "no";
  value?: string;
};

const tiers: Tier[] = [
  {
    name: "Developer",
    purpose: "Prototype",
    description: "Efficient access for early-stage agentic systems.",
    monthly: {
      amount: "Free",
      cadence: "",
      note: "For personal environments and validation loops.",
    },
    yearly: {
      amount: "Free",
      cadence: "",
      note: "For personal environments and validation loops.",
    },
    features: [
      { label: "1,000 Audit Trails/mo" },
      { label: "Basic Belief Graph (3-level depth)" },
      { label: "Community Support" },
    ],
  },
  {
    name: "Pro",
    purpose: "Production Teams",
    description: "Operational headroom for teams moving from pilots to live workloads.",
    featured: true,
    monthly: {
      amount: "$499",
      cadence: "/mo",
      note: "Month-to-month for active production teams.",
    },
    yearly: {
      amount: "$449",
      cadence: "/mo",
      note: "Efficient annual billing, billed at $5,388 per year.",
    },
    features: [
      { label: "50,000 Audit Trails/mo" },
      { label: "Full Causal Collapse Engine", italic: true },
      { label: "5 eBPF Managed Agents" },
      { label: "14-day Trace Retention" },
    ],
  },
  {
    name: "Enterprise",
    purpose: "Banks & Healthcare",
    description: "Structured contracts for regulated, kernel-adjacent deployments.",
    monthly: {
      amount: "Custom",
      cadence: "",
      note: "Commercial architecture review required.",
    },
    yearly: {
      amount: "Custom",
      cadence: "",
      note: "Annual deployment agreements scoped with your controls team.",
    },
    features: [
      { label: "Unlimited Reasoning States" },
      { label: "Kernel-level Interception" },
      { label: "SOC2 Audit Export" },
      { label: "7-year Immutable Archiving" },
      { label: "Dedicated Logic Architect" },
    ],
  },
];

const comparisonRows: Array<{
  label: string;
  values: [ComparisonCell, ComparisonCell, ComparisonCell];
}> = [
  {
    label: "Logic Depth",
    values: [
      { kind: "text", value: "3-level graph" },
      { kind: "text", value: "Full collapse engine" },
      { kind: "text", value: "Unlimited states" },
    ],
  },
  {
    label: "eBPF Integration",
    values: [
      { kind: "no" },
      { kind: "text", value: "5 managed agents" },
      { kind: "text", value: "Kernel-level interception" },
    ],
  },
  {
    label: "Trace Persistence",
    values: [
      { kind: "text", value: "Session scoped" },
      { kind: "text", value: "14 days" },
      { kind: "text", value: "7-year immutable archive" },
    ],
  },
  {
    label: "Compliance Export",
    values: [
      { kind: "no" },
      { kind: "no" },
      { kind: "yes" },
    ],
  },
  {
    label: "Custom Belief Schemas",
    values: [
      { kind: "no" },
      { kind: "no" },
      { kind: "yes" },
    ],
  },
];

export const metadata: Metadata = {
  title: "Pricing | Velarix",
  description:
    "Pricing for every agentic scale, from prototypes to kernel-level enterprise deployments.",
};

function getBillingCycle(
  billing: string | string[] | undefined,
): BillingCycle {
  const value = Array.isArray(billing) ? billing[0] : billing;
  return value === "yearly" ? "yearly" : "monthly";
}

function StatusMark({ cell }: { cell: ComparisonCell }) {
  if (cell.kind === "yes") {
    return (
      <span className="inline-flex items-center justify-center rounded-full border border-white/10 bg-white/[0.03] p-1 text-primary-accent-chrome">
        <Check className="h-3.5 w-3.5" strokeWidth={2.2} />
        <span className="sr-only">Included</span>
      </span>
    );
  }

  if (cell.kind === "no") {
    return (
      <span className="inline-flex items-center justify-center rounded-full border border-white/10 bg-white/[0.02] p-2">
        <span className="h-1.5 w-1.5 rounded-full bg-zinc-500" />
        <span className="sr-only">Not included</span>
      </span>
    );
  }

  return (
    <span className="text-[13px] tracking-[-0.01em] text-zinc-200">
      {cell.value}
    </span>
  );
}

function AccentFor({ children }: { children: ReactNode }) {
  return (
    <span
      className={`${instrumentSerif.className} italic text-white/94`}
    >
      {children}
    </span>
  );
}

export default async function PricingPage({
  searchParams,
}: PricingPageProps) {
  const resolvedSearchParams = searchParams ? await searchParams : undefined;
  const billing = getBillingCycle(resolvedSearchParams?.billing);

  return (
    <main className="relative min-h-screen overflow-hidden bg-background text-text-primary">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(45,132,247,0.14),transparent_34%),radial-gradient(circle_at_78%_18%,rgba(199,208,219,0.08),transparent_18%)]" />
      <div className="absolute inset-x-0 top-0 h-px bg-white/5" />

      <div className="relative mx-auto max-w-[1320px] px-4 pb-24 pt-10 sm:px-6 sm:pb-28 lg:px-8 lg:pb-36 lg:pt-14">
        <section className="rounded-[32px] border border-white/5 bg-zinc-950/20 px-6 py-16 sm:px-8 sm:py-20 lg:rounded-[40px] lg:px-12 lg:py-24">
          <div className="mx-auto max-w-[920px] text-center">
            <p className="font-mono text-[11px] uppercase tracking-[0.28em] text-zinc-500">
              Institutional Pricing
            </p>
            <h1
              className={`${geist.className} mx-auto mt-8 max-w-[12ch] text-5xl font-bold tracking-[-0.07em] text-white sm:text-7xl lg:text-[5.85rem] lg:leading-[0.95]`}
            >
              Pricing <AccentFor>for</AccentFor> Every Agentic Scale.
            </h1>
            <p className="mx-auto mt-8 max-w-[60ch] text-base leading-8 tracking-[-0.01em] text-zinc-400 sm:text-[1.55rem] sm:leading-[2.3rem]">
              From early-stage prototypes to kernel-level enterprise
              deployments. No hidden logic, just scale.
            </p>

            <div className="mt-14 flex justify-center">
              <div className="inline-flex items-center gap-1 rounded-full border border-white/5 bg-white/[0.03] p-1.5">
                {(["monthly", "yearly"] as const).map((option) => {
                  const isActive = billing === option;

                  return (
                    <Link
                      key={option}
                      href={
                        option === "monthly"
                          ? "/pricing"
                          : "/pricing?billing=yearly"
                      }
                      className={`rounded-full px-5 py-2.5 text-sm font-medium tracking-[-0.02em] ${
                        isActive
                          ? "bg-white text-black"
                          : "bg-white/5 text-zinc-400 hover:text-zinc-200"
                      }`}
                      aria-current={isActive ? "page" : undefined}
                    >
                      {option === "monthly" ? "Monthly" : "Yearly"}
                    </Link>
                  );
                })}
              </div>
            </div>

            <p className="mt-5 text-sm tracking-[-0.01em] text-zinc-500">
              {billing === "monthly"
                ? "Flexible monthly procurement for active deployment teams."
                : "Yearly contracts emphasize efficient spend and longer operational windows."}
            </p>
          </div>
        </section>

        <section className="pt-20 sm:pt-24 lg:pt-28">
          <div className="grid gap-6 lg:grid-cols-3">
            {tiers.map((tier) => {
              const price = tier[billing];

              return (
                <article
                  key={tier.name}
                  className="flex h-full flex-col rounded-[32px] border border-white/5 bg-zinc-950/20 p-7 sm:p-8"
                  style={
                    tier.featured
                      ? {
                          boxShadow:
                            "0 0 0 1px rgba(143, 196, 255, 0.14), 0 0 42px rgba(45, 132, 247, 0.14)",
                        }
                      : undefined
                  }
                >
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="font-mono text-[11px] uppercase tracking-[0.24em] text-zinc-500">
                        {tier.purpose}
                      </p>
                      <h2
                        className={`${geist.className} mt-4 text-[2rem] font-semibold tracking-[-0.05em] text-white`}
                      >
                        {tier.name}
                      </h2>
                    </div>
                    {tier.featured ? (
                      <span className="rounded-full border border-logic-blue-400/20 bg-logic-blue-500/10 px-3 py-1.5 text-[11px] font-medium tracking-[0.08em] text-logic-blue-200">
                        Most Efficient
                      </span>
                    ) : null}
                  </div>

                  <p className="mt-4 max-w-[34ch] text-sm leading-7 text-zinc-400">
                    {tier.description}
                  </p>

                  <div className="mt-10 border-t border-white/5 pt-8">
                    <div className="flex items-end gap-2">
                      <span
                        className={`${geist.className} text-4xl font-bold tracking-[-0.06em] text-white sm:text-5xl`}
                      >
                        {price.amount}
                      </span>
                      {price.cadence ? (
                        <span className="pb-1 text-base tracking-[-0.02em] text-zinc-500">
                          {price.cadence}
                        </span>
                      ) : null}
                    </div>
                    <p className="mt-3 text-sm leading-6 text-zinc-500">
                      {price.note}
                    </p>
                  </div>

                  <ul className="mt-10 space-y-4 border-t border-white/5 pt-8 text-sm tracking-[-0.01em] text-zinc-200">
                    {tier.features.map((feature) => (
                      <li key={feature.label} className="flex items-start gap-3">
                        <span className="mt-2 h-1.5 w-1.5 rounded-full bg-primary-accent-chrome" />
                        <span
                          className={
                            feature.italic
                              ? `${instrumentSerif.className} italic`
                              : undefined
                          }
                        >
                          {feature.label}
                        </span>
                      </li>
                    ))}
                  </ul>
                </article>
              );
            })}
          </div>
        </section>

        <section className="pt-20 sm:pt-24 lg:pt-28">
          <div className="rounded-[36px] border border-white/5 bg-zinc-950/20 p-4 sm:p-5 lg:p-6">
            <div className="px-3 pb-8 pt-4 sm:px-5">
              <p className="font-mono text-[11px] uppercase tracking-[0.24em] text-zinc-500">
                Technical Comparison
              </p>
              <h2
                className={`${geist.className} mt-5 text-3xl font-semibold tracking-[-0.05em] text-white sm:text-4xl`}
              >
                Institutional clarity at every deployment tier.
              </h2>
            </div>

            <div className="overflow-hidden rounded-[28px] border border-white/5">
              <table className="w-full border-collapse text-left text-[13px]">
                <thead className="bg-white/[0.03]">
                  <tr>
                    <th className="border-b border-r border-white/5 px-4 py-4 font-medium text-zinc-500 sm:px-5">
                      Capability
                    </th>
                    {tiers.map((tier) => (
                      <th
                        key={tier.name}
                        className="border-b border-r border-white/5 px-4 py-4 font-medium text-zinc-200 last:border-r-0 sm:px-5"
                      >
                        {tier.name}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {comparisonRows.map((row) => (
                    <tr key={row.label} className="bg-zinc-950/20">
                      <th className="border-b border-r border-white/5 px-4 py-4 font-medium tracking-[-0.01em] text-zinc-300 sm:px-5">
                        {row.label}
                      </th>
                      {row.values.map((cell, index) => (
                        <td
                          key={`${row.label}-${tiers[index].name}`}
                          className="border-b border-r border-white/5 px-4 py-4 align-middle text-zinc-200 last:border-r-0 last:border-b-0 sm:px-5"
                        >
                          <StatusMark cell={cell} />
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </section>
      </div>
    </main>
  );
}
