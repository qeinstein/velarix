"use client";

import type { ReactNode } from "react";
import { useRef, useState } from "react";

import {
  ArrowRight,
  BookOpenText,
  ChevronDown,
  Command,
  KeyRound,
  LockKeyhole,
  Orbit,
  ShieldCheck,
  Waypoints,
  Workflow,
} from "lucide-react";
import {
  MotionConfig,
  motion,
  useInView,
  useReducedMotion,
} from "motion/react";
import Link from "next/link";

const mechanicalEase: [number, number, number, number] = [0.16, 1, 0.3, 1];

const navItems = [
  { label: "Products", href: "#products" },
  { label: "Solutions", href: "#solutions" },
  { label: "Resources", href: "#resources" },
  { label: "Pricing", href: "/pricing" },
  { label: "Documentation", href: "/docs" },
  { label: "Blog", href: "#blog" },
];

const installCommands = [
  { label: "Python SDK", command: "pip install velarix" },
  { label: "Node.js SDK", command: "npm install velarix-sdk" },
];

const codeExamples = {
  python: {
    label: "Python",
    install: "pip install velarix",
    lines: [
      "import os",
      "",
      "from velarix.adapters.openai import OpenAI",
      "",
      "client = OpenAI(",
      '    api_key=os.environ["OPENAI_API_KEY"],',
      '    velarix_base_url=os.environ["VELARIX_BASE_URL"],',
      '    velarix_session_id="claims-prod",',
      ")",
      "",
      "response = client.chat.completions.create(",
      '    model="gpt-4o",',
      '    messages=[{"role": "user", "content": "Review this claim for manual escalation."}],',
      ")",
    ],
  },
  node: {
    label: "Node.js",
    install: "npm install velarix-sdk",
    lines: [
      'import OpenAI from "openai";',
      'import { VelarixClient, VelarixOpenAI } from "velarix-sdk";',
      "",
      "const velarix = new VelarixClient({",
      "  baseUrl: process.env.VELARIX_BASE_URL,",
      "  apiKey: process.env.VELARIX_API_KEY,",
      "});",
      "",
      'const openai = new OpenAI({ apiKey: process.env.OPENAI_API_KEY });',
      "const client = new VelarixOpenAI(openai, velarix);",
      "",
      'const response = await client.chat("claims-prod", {',
      '  model: "gpt-4o",',
      '  messages: [{ role: "user", content: "Review this claim for manual escalation." }],',
      "});",
    ],
  },
} as const;

const capabilityCards = [
  {
    icon: Orbit,
    title: "Kernel-level interception",
    body:
      "Intercept agent actions at the system boundary before they turn into production events.",
  },
  {
    icon: Waypoints,
    title: "Causal Graph Validation",
    body:
      "Validate decisions against justified state instead of reconstructing lineage after the fact.",
  },
  {
    icon: Workflow,
    title: "Deterministic State Management",
    body:
      "Keep state transitions explicit, inspectable, and repeatable across every session.",
  },
  {
    icon: LockKeyhole,
    title: "Zero-Trust Agentic Security",
    body:
      "Enforce scoped access, isolated sessions, and verifiable traces by default.",
  },
];

const resourceCards = [
  {
    icon: BookOpenText,
    eyebrow: "Docs",
    title: "Integration guide",
    body: "Deployment guidance for setup, rollout, and runtime operations.",
    meta: "Documentation",
  },
  {
    icon: Command,
    eyebrow: "SDKs",
    title: "Provider adapters",
    body: "Production-ready SDK flows for Python and Node.js.",
    meta: "Python and Node.js",
  },
  {
    icon: ShieldCheck,
    eyebrow: "Security",
    title: "Threat model",
    body: "Security boundaries and controls for high-trust deployments.",
    meta: "Enterprise security",
  },
];

const deploymentCards = [
  {
    name: "Starter",
    body: "Start with SDK adoption inside existing agent workflows.",
    points: "Python and Node.js SDKs with session-aware execution.",
  },
  {
    name: "Platform",
    body: "Roll out validated state and traceable execution across products.",
    points: "Key management, session history, and reasoning retrieval.",
  },
  {
    name: "Enterprise",
    body: "Standardize on security-reviewed agent infrastructure.",
    points: "Policy alignment, audit readiness, and kernel-level controls.",
  },
];

const blogCards = [
  {
    title: "LangChain adapter patterns",
    body: "Connect validated state to tool-aware execution.",
    meta: "Integration notes",
  },
  {
    title: "LangGraph memory routing",
    body: "Replace opaque memory with justified state transitions.",
    meta: "Architecture",
  },
  {
    title: "LlamaIndex retrieval hooks",
    body: "Bring validated slices into retrieval workflows.",
    meta: "Retrieval",
  },
];

const codeKeywords = new Set([
  "import",
  "from",
  "const",
  "new",
  "await",
  "async",
  "return",
]);

type CodeTab = keyof typeof codeExamples;

function Reveal({
  children,
  className,
  delay = 0,
}: {
  children: ReactNode;
  className?: string;
  delay?: number;
}) {
  const shouldReduceMotion = useReducedMotion();

  return (
    <motion.div
      initial={{ opacity: 0, y: shouldReduceMotion ? 0 : 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.2 }}
      transition={{ duration: 0.4, delay, ease: mechanicalEase }}
      className={className}
    >
      {children}
    </motion.div>
  );
}

function EditorialAccent({ children }: { children: ReactNode }) {
  return (
    <span className="font-serif text-[0.98em] font-medium italic text-white/96">
      {children}
    </span>
  );
}

function AnnouncementBadge() {
  return (
    <div className="inline-flex items-center gap-3 rounded-full border border-white/10 bg-zinc-950/50 px-4 py-1.5 text-sm text-zinc-300">
      <span className="relative flex h-2.5 w-2.5 items-center justify-center">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400/60" />
        <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-400" />
      </span>
      <span>
        Audit every reasoning step <EditorialAccent>in</EditorialAccent>{" "}
        real-time.
      </span>
    </div>
  );
}

function Logo() {
  return (
    <div className="flex items-center gap-3">
      <div className="relative flex h-9 w-9 items-center justify-center rounded-full border border-white/12 bg-white/[0.04]">
        <div className="absolute h-4 w-4 rotate-45 border border-white/45" />
        <div className="h-1.5 w-1.5 rounded-full bg-white" />
      </div>
      <div>
        <p className="text-sm font-semibold tracking-[-0.03em] text-white">
          Velarix
        </p>
        <p className="font-mono text-[10px] uppercase tracking-[0.22em] text-zinc-500">
          Infrastructure
        </p>
      </div>
    </div>
  );
}

function HeaderLink({ label, href }: { label: string; href: string }) {
  const showChevron = href.startsWith("#");

  return (
    <Link
      href={href}
      className="subtle-link inline-flex items-center gap-1.5 rounded-full px-3 py-2 text-sm tracking-[-0.01em] text-zinc-400"
    >
      <span>{label}</span>
      {showChevron ? <ChevronDown className="h-3.5 w-3.5 text-zinc-600" /> : null}
    </Link>
  );
}

function renderCodeToken(token: string, key: string) {
  if (!token) {
    return null;
  }

  if (token.startsWith("#") || token.startsWith("//")) {
    return (
      <span key={key} className="text-zinc-500">
        {token}
      </span>
    );
  }

  if (token.startsWith('"') || token.startsWith("'")) {
    return (
      <span key={key} className="text-logic-blue-200">
        {token}
      </span>
    );
  }

  if (codeKeywords.has(token)) {
    return (
      <span key={key} className="text-primary-accent-chrome">
        {token}
      </span>
    );
  }

  if (/^[A-Z][A-Za-z0-9_]+$/.test(token)) {
    return (
      <span key={key} className="text-white">
        {token}
      </span>
    );
  }

  return <span key={key}>{token}</span>;
}

function highlightCode(line: string) {
  if (!line.trim()) {
    return <span>&nbsp;</span>;
  }

  const tokens = line.split(
    /(#.*$|\/\/.*$|"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|\b(?:import|from|const|new|await|async|return)\b)/g,
  );

  return tokens.map((token, index) =>
    renderCodeToken(token, `${line}-${index}`),
  );
}

function TypedCodeBlock({
  code,
  shouldReduceMotion,
}: {
  code: (typeof codeExamples)[CodeTab];
  shouldReduceMotion: boolean;
}) {
  const blockRef = useRef<HTMLDivElement | null>(null);
  const inView = useInView(blockRef, { once: true, amount: 0.35 });

  return (
    <div
      ref={blockRef}
      className="overflow-hidden rounded-[32px] border border-white/5 bg-[rgba(10,10,10,0.92)]"
    >
      <div className="flex items-center justify-between gap-3 border-b border-white/5 px-6 py-5">
        <div>
          <p className="text-sm font-semibold tracking-[-0.02em] text-white">
            {code.label} runtime
          </p>
          <p className="mt-1 font-mono text-[11px] uppercase tracking-[0.18em] text-zinc-500">
            {code.install}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="h-2.5 w-2.5 rounded-full bg-danger/80" />
          <span className="h-2.5 w-2.5 rounded-full bg-warning/80" />
          <span className="h-2.5 w-2.5 rounded-full bg-success/80" />
        </div>
      </div>

      <div className="overflow-x-auto px-6 py-6">
        <pre className="min-w-[620px] font-mono text-[13px] leading-[24px] text-zinc-300">
          {code.lines.map((line, index) => (
            <motion.span
              key={`${code.label}-${index}`}
              initial={
                shouldReduceMotion
                  ? { opacity: 1 }
                  : { opacity: 0, y: 6, clipPath: "inset(0 100% 0 0)" }
              }
              animate={
                shouldReduceMotion
                  ? { opacity: 1 }
                  : inView
                    ? { opacity: 1, y: 0, clipPath: "inset(0 0% 0 0)" }
                    : undefined
              }
              transition={{
                duration: 0.34,
                delay: shouldReduceMotion ? 0 : index * 0.055,
                ease: mechanicalEase,
              }}
              className="block whitespace-pre"
            >
              {highlightCode(line)}
            </motion.span>
          ))}
        </pre>
      </div>
    </div>
  );
}

function CodeExperience() {
  const shouldReduceMotion = useReducedMotion();
  const [activeTab, setActiveTab] = useState<CodeTab>("python");
  const activeCode = codeExamples[activeTab];

  return (
    <div>
      <div>
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <p className="text-base font-semibold tracking-[-0.02em] text-white">
              Step 03. Start Building
            </p>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-zinc-400">
              Production quickstarts for Python and Node.js.
            </p>
          </div>
          <div className="flex items-center gap-2">
            {Object.entries(codeExamples).map(([key, item]) => {
              const selected = key === activeTab;

              return (
                <button
                  key={key}
                  type="button"
                  onClick={() => setActiveTab(key as CodeTab)}
                  className={`rounded-full px-4 py-2 text-sm font-medium tracking-[-0.01em] ${
                    selected
                      ? "bg-white text-black"
                      : "border border-white/10 text-zinc-400 hover:text-zinc-200"
                  }`}
                  aria-pressed={selected}
                >
                  {item.label}
                </button>
              );
            })}
          </div>
        </div>

        <div className="mt-6">
          <TypedCodeBlock
            key={activeTab}
            code={activeCode}
            shouldReduceMotion={Boolean(shouldReduceMotion)}
          />
        </div>
      </div>
    </div>
  );
}

function StepCard({
  number,
  title,
  body,
  children,
  isLast = false,
}: {
  number: string;
  title: string;
  body: string;
  children: ReactNode;
  isLast?: boolean;
}) {
  return (
    <li className="relative grid gap-6 md:grid-cols-[72px_minmax(0,1fr)] md:gap-10">
      <div className="relative z-10 flex h-14 items-center">
        <div className="inline-flex h-14 w-14 items-center justify-center rounded-full border border-white/10 bg-black font-mono text-sm tracking-[0.22em] text-zinc-300">
          {number}
        </div>
      </div>
      <Reveal
        className={`pb-10 ${isLast ? "" : "border-b border-white/6"} sm:pb-12`}
      >
        <div className="flex flex-col gap-4">
          <div>
            <p className="text-2xl font-semibold tracking-[-0.04em] text-white">
              {title}
            </p>
            <p className="mt-3 max-w-3xl text-base leading-7 text-zinc-400">
              {body}
            </p>
          </div>
          {children}
        </div>
      </Reveal>
    </li>
  );
}

function CapabilityCard({
  icon: Icon,
  title,
  body,
}: {
  icon: typeof Orbit;
  title: string;
  body: string;
}) {
  return (
    <Reveal className="group rounded-[32px] border border-white/5 bg-white/[0.02] p-7">
      <div className="flex h-full flex-col gap-6">
        <div className="inline-flex h-11 w-11 items-center justify-center border border-white/10 bg-black text-zinc-100 transition-colors duration-200 group-hover:text-white">
          <Icon className="h-5 w-5" />
        </div>
        <div>
          <p className="text-lg font-semibold tracking-[-0.025em] text-white">
            {title}
          </p>
          <p className="mt-3 text-sm leading-7 text-zinc-400">{body}</p>
        </div>
      </div>
    </Reveal>
  );
}

export default function LandingPage() {
  const shouldReduceMotion = useReducedMotion();

  return (
    <MotionConfig transition={{ duration: 0.18, ease: mechanicalEase }}>
      <main className="relative overflow-x-clip bg-[#0A0A0C] text-white">
        <motion.header
          initial={{ opacity: 0, y: shouldReduceMotion ? 0 : -18 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.28, ease: mechanicalEase }}
          className="sticky top-0 z-40 border-b border-white/5 bg-[#0A0A0C]/80 backdrop-blur-md"
        >
          <div className="flex min-h-18 w-full items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
                <a href="#top" className="rounded-full px-2 py-1">
                  <Logo />
                </a>

                <nav className="hidden items-center gap-1 lg:flex">
                  {navItems.map((item) => (
                    <HeaderLink key={item.label} label={item.label} href={item.href} />
                  ))}
                </nav>

                <div className="flex items-center gap-2">
                  <Link
                    href="/docs"
                    className="shimmer-button inline-flex items-center justify-center rounded-full bg-white px-5 py-2.5 text-sm font-semibold text-black"
                  >
                    Dashboard
                  </Link>
                  <div className="flex h-10 w-10 items-center justify-center rounded-full border border-white/12 bg-white/[0.04] font-mono text-[11px] uppercase tracking-[0.18em] text-zinc-300">
                    VX
                  </div>
                </div>
          </div>
        </motion.header>

        <div className="relative mx-auto max-w-[1380px] px-4 pb-24 sm:px-6 lg:px-8">
          <section id="top" className="pb-32 pt-8 sm:pt-10 lg:pb-36 lg:pt-12">
            <div className="mx-auto max-w-5xl text-center">
              <Reveal className="mt-16 flex flex-col items-center">
                <AnnouncementBadge />
                <h1 className="mt-8 max-w-[15ch] text-5xl font-bold leading-[1.1] tracking-[-0.04em] text-white sm:text-7xl lg:text-[5.9rem]">
                  The Logic Layer <EditorialAccent>for</EditorialAccent>{" "}
                  Autonomous Agents.
                </h1>
                <p className="mt-8 max-w-2xl text-lg leading-relaxed tracking-[-0.01em] text-zinc-500 sm:text-xl">
                  Velarix gives agents verified state, causal traceability, and
                  controlled execution.
                </p>

                <div className="mt-12 flex flex-col items-center gap-4 sm:flex-row">
                  <a
                    href="#products"
                    className="shimmer-button inline-flex items-center justify-center gap-2 rounded-full bg-white px-6 py-3 text-sm font-semibold text-black"
                  >
                    Infrastructure in Minutes
                    <ArrowRight className="h-4 w-4" />
                  </a>
                  <Link
                    href="/docs"
                    className="shimmer-button inline-flex items-center justify-center gap-2 rounded-full border border-white/10 bg-white/[0.03] px-6 py-3 text-sm font-medium text-zinc-100 hover:text-zinc-300"
                  >
                    Read Documentation
                  </Link>
                </div>
              </Reveal>

            </div>
          </section>

          <section id="products" className="py-32">
            <div className="mx-auto max-w-6xl">
              <Reveal className="max-w-4xl">
                <h2 className="mt-8 text-4xl leading-[1.02] font-semibold tracking-[-0.05em] text-white sm:text-5xl lg:text-6xl">
                  Deploy Velarix Fast
                </h2>
                <p className="mt-6 max-w-3xl text-lg leading-8 text-zinc-400">
                  Go from install to live reasoning workflows in three steps.
                </p>
              </Reveal>

              <ol className="relative mt-20 space-y-10 before:absolute before:left-7 before:top-6 before:bottom-6 before:w-px before:bg-white/10 md:space-y-14 md:before:left-9">
                <StepCard
                  number="01"
                  title="Install the SDK"
                  body="Install the SDK, pair it with your model provider, and connect the session you want to verify."
                >
                  <div className="grid gap-6 sm:grid-cols-2">
                    {installCommands.map((item) => (
                      <div key={item.label}>
                        <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-zinc-500">
                          {item.label}
                        </p>
                        <pre className="mt-2 overflow-x-auto font-mono text-[14px] leading-6 text-zinc-100">
                          {item.command}
                        </pre>
                      </div>
                    ))}
                  </div>
                </StepCard>

                <StepCard
                  number="02"
                  title="Get your API key"
                  body="Create scoped credentials in the dashboard and bind them to the environments where sessions run."
                >
                  <div className="grid gap-3 text-sm text-zinc-300 md:grid-cols-3">
                    {[
                      "Workspace-scoped keys",
                      "Rotation and revocation",
                      "Per-environment base URLs",
                    ].map((item) => (
                      <div key={item} className="flex items-center gap-3">
                        <KeyRound className="h-4 w-4 text-zinc-400" />
                        <p className="font-medium tracking-[-0.01em] text-zinc-200">
                          {item}
                        </p>
                      </div>
                    ))}
                  </div>
                </StepCard>

                <StepCard
                  number="03"
                  title="Start building"
                  body="Use the runtime your team already ships and move straight into execution, retrieval, and explanation."
                  isLast
                >
                  <CodeExperience />
                </StepCard>
              </ol>
            </div>
          </section>

          <section id="solutions" className="py-32">
            <div className="mx-auto max-w-6xl">
              <Reveal className="max-w-4xl">
                <h2 className="mt-8 text-4xl leading-[1.04] font-semibold tracking-[-0.05em] text-white sm:text-5xl">
                  Built <EditorialAccent>for</EditorialAccent> stateful
                  production systems.
                </h2>
                <p className="mt-6 max-w-3xl text-lg leading-8 text-zinc-400">
                  Use Velarix where model output must map to durable state and
                  an auditable decision path.
                </p>
              </Reveal>

              <div className="mt-16 grid gap-5 lg:grid-cols-2 xl:grid-cols-4">
                {capabilityCards.map((card) => (
                  <CapabilityCard key={card.title} {...card} />
                ))}
              </div>
            </div>
          </section>

          <section id="resources" className="py-32">
            <div className="mx-auto max-w-6xl">
              <Reveal className="max-w-4xl">
                <h2 className="mt-8 text-4xl leading-[1.04] font-semibold tracking-[-0.05em] text-white sm:text-5xl">
                  Resources <EditorialAccent>for</EditorialAccent> teams
                  shipping now.
                </h2>
              </Reveal>

              <div className="mt-16 grid gap-5 lg:grid-cols-3">
                {resourceCards.map((card) => {
                  const Icon = card.icon;

                  return (
                    <Reveal
                      key={card.title}
                      className="rounded-[32px] border border-white/5 bg-white/[0.02] p-7"
                    >
                      <div className="flex h-full flex-col gap-5">
                        <div className="inline-flex h-11 w-11 items-center justify-center border border-white/10 bg-black text-zinc-200">
                          <Icon className="h-5 w-5" />
                        </div>
                        <div>
                          <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-zinc-500">
                            {card.eyebrow}
                          </p>
                          <p className="mt-3 text-xl font-semibold tracking-[-0.03em] text-white">
                            {card.title}
                          </p>
                          <p className="mt-3 text-sm leading-7 text-zinc-400">
                            {card.body}
                          </p>
                        </div>
                        <p className="font-mono text-[12px] text-zinc-400">
                          {card.meta}
                        </p>
                      </div>
                    </Reveal>
                  );
                })}
              </div>
            </div>
          </section>

          <section id="pricing" className="py-32">
            <div className="mx-auto max-w-6xl">
              <Reveal className="max-w-4xl">
                <h2 className="mt-8 text-4xl leading-[1.04] font-semibold tracking-[-0.05em] text-white sm:text-5xl">
                  Deployment paths <EditorialAccent>for</EditorialAccent>{" "}
                  operational maturity.
                </h2>
                <p className="mt-6 max-w-3xl text-lg leading-8 text-zinc-400">
                  Scale from SDK adoption to platform rollout without changing
                  the operating model.
                </p>
              </Reveal>

              <div className="mt-16 grid gap-5 lg:grid-cols-3">
                {deploymentCards.map((card) => (
                  <Reveal
                    key={card.name}
                    className="rounded-[32px] border border-white/5 bg-white/[0.02] p-7"
                  >
                    <p className="font-mono text-[11px] uppercase tracking-[0.2em] text-zinc-500">
                      {card.name}
                    </p>
                    <p className="mt-4 text-2xl font-semibold tracking-[-0.035em] text-white">
                      {card.body}
                    </p>
                    <p className="mt-6 text-sm leading-7 text-zinc-400">
                      {card.points}
                    </p>
                  </Reveal>
                ))}
              </div>
            </div>
          </section>

          <section id="documentation" className="py-32">
            <div className="mx-auto max-w-6xl">
              <Reveal className="rounded-[32px] border border-white/5 bg-white/[0.02] p-8 sm:p-10 lg:p-12">
                <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_260px] lg:items-end">
                  <div>
                    <h2 className="mt-8 max-w-3xl text-4xl leading-[1.04] font-semibold tracking-[-0.05em] text-white sm:text-5xl">
                      Documentation <EditorialAccent>for</EditorialAccent>{" "}
                      production rollout.
                    </h2>
                    <p className="mt-6 max-w-3xl text-lg leading-8 text-zinc-400">
                      Setup, security, and integration guidance for teams that
                      need precision.
                    </p>
                  </div>

                  <div className="flex flex-col gap-3">
                    <Link
                      href="/docs"
                      className="shimmer-button inline-flex items-center justify-between rounded-full bg-white px-5 py-3 text-sm font-semibold text-black"
                    >
                      Open Technical Resources
                      <ArrowRight className="h-4 w-4" />
                    </Link>
                    <a
                      href="#blog"
                      className="shimmer-button inline-flex items-center justify-between rounded-full border border-white/10 bg-white/[0.03] px-5 py-3 text-sm font-medium text-zinc-100 hover:text-zinc-300"
                    >
                      View Integration Notes
                      <ArrowRight className="h-4 w-4" />
                    </a>
                  </div>
                </div>
              </Reveal>
            </div>
          </section>

          <section id="blog" className="py-32">
            <div className="mx-auto max-w-6xl">
              <Reveal className="max-w-4xl">
                <h2 className="mt-8 text-4xl leading-[1.04] font-semibold tracking-[-0.05em] text-white sm:text-5xl">
                  Integration notes <EditorialAccent>for</EditorialAccent>{" "}
                  production teams.
                </h2>
              </Reveal>

              <div className="mt-16 grid gap-5 lg:grid-cols-3">
                {blogCards.map((card) => (
                  <Reveal
                    key={card.title}
                    className="rounded-[32px] border border-white/5 bg-white/[0.02] p-7"
                  >
                    <div className="flex h-full flex-col justify-between gap-6">
                      <div>
                        <p className="text-xl font-semibold tracking-[-0.03em] text-white">
                          {card.title}
                        </p>
                        <p className="mt-3 text-sm leading-7 text-zinc-400">
                          {card.body}
                        </p>
                      </div>
                      <p className="font-mono text-[12px] text-zinc-400">
                        {card.meta}
                      </p>
                    </div>
                  </Reveal>
                ))}
              </div>
            </div>
          </section>
        </div>
      </main>
    </MotionConfig>
  );
}
