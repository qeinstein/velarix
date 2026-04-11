import CodeSwapPanel from "../../components/CodeSwapPanel";

const quickNotes = [
  {
    label: "Integration",
    title: "One import swap",
    description:
      "Keep your existing OpenAI workflow and add a session id when memory needs to persist.",
  },
  {
    label: "Correction",
    title: "Prune stale chains",
    description:
      "A changed premise invalidates the reasoning built on top of it instead of silently lingering.",
  },
  {
    label: "Deployment",
    title: "Self-host or cloud",
    description:
      "Run it locally for free or move to managed sessions when you need shared state and billing.",
  },
];

const capabilities = [
  {
    title: "Correct stale reasoning",
    description:
      "Velarix tracks causal dependencies, so one updated fact invalidates every conclusion that depended on it — automatically.",
  },
  {
    title: "Show the why-trace",
    description:
      "Every decision can be traced back through the beliefs that produced it. Debugging stops being theatrical.",
  },
  {
    title: "Gate before execution",
    description:
      "The execute-check API blocks stale decisions before they run. If a dependency changed, the action does not proceed.",
  },
  {
    title: "Stay close to the runtime",
    description:
      "The surface is intentionally small. No new orchestration layer — just a session id and a changed import.",
  },
  {
    title: "Full audit trail",
    description:
      "Every fact assertion, invalidation, and decision is journalled. Compliance export ships out of the box.",
  },
  {
    title: "Pluggable storage",
    description:
      "BadgerDB for local development. PostgreSQL + Redis for production. Switch with a single environment variable.",
  },
];

const plans = [
  {
    name: "Self-hosted",
    price: "$0",
    cadence: "forever",
    detail: "Open core",
    items: ["Run with Docker", "Local BadgerDB storage", "Own the full stack", "No usage limits"],
    href: "https://github.com/qeinstein/velarix",
    label: "View source",
    external: true,
    featured: false,
  },
  {
    name: "Cloud",
    price: "$29",
    cadence: "per month",
    detail: "Managed",
    items: [
      "Persistent sessions",
      "PostgreSQL + Redis backend",
      "Org controls and invitations",
      "Compliance export",
    ],
    href: "/signup",
    label: "Get started",
    external: false,
    featured: true,
  },
  {
    name: "Enterprise",
    price: "Custom",
    cadence: "flexible",
    detail: "Dedicated",
    items: [
      "Custom SLAs",
      "On-premise deployment",
      "Dedicated support",
      "Custom retention and policy",
    ],
    href: "mailto:hello@velarix.com",
    label: "Contact sales",
    external: true,
    featured: false,
  },
];

const integrations = [
  "LangGraph",
  "CrewAI",
  "LlamaIndex",
  "LangChain",
  "OpenAI",
  "Any HTTP client",
];

export default function Home() {
  const year = new Date().getFullYear();

  return (
    <main className="pb-24 pt-10 md:pt-16">
      {/* Hero */}
      <section className="grid gap-14 lg:grid-cols-[minmax(0,1.45fr)_minmax(18rem,0.7fr)] lg:gap-16">
        <div className="space-y-8">
          <p className="eyebrow">Causal memory for AI agents</p>
          <h1 className="font-display max-w-4xl text-[clamp(4rem,11vw,8.25rem)] leading-[0.88] tracking-[-0.08em]">
            Memory you can correct.
          </h1>
          <div className="copy-tone font-copy max-w-2xl space-y-4 text-[1.16rem] leading-8">
            <p>
              Velarix gives agents a causal memory layer. When a fact changes, stale reasoning is
              pruned instead of quietly surviving in context.
            </p>
            <p>
              The point is not more interface. The point is less ambiguity in the runtime your
              agents already use.
            </p>
          </div>
          <div className="flex flex-wrap gap-3 pt-2">
            <a href="/signup" className="button-solid">
              Get started
            </a>
            <a href="/docs" className="button-ghost">
              Read docs
            </a>
          </div>
        </div>

        <aside className="space-y-8 border-t border-[var(--line)] pt-4 lg:border-l lg:border-t-0 lg:pl-8 lg:pt-0">
          <div className="space-y-3">
            <p className="eyebrow">Why it exists</p>
            <p className="font-copy text-lg leading-7 text-[var(--muted)]">
              Models are good at continuation. Velarix is the layer that notices when the premise
              changed and removes the downstream fiction.
            </p>
          </div>
          <div className="grid gap-4 sm:grid-cols-3 lg:grid-cols-1">
            {quickNotes.map((note) => (
              <div key={note.label} className="border-t border-[var(--line)] pt-3">
                <p className="eyebrow">{note.label}</p>
                <h2 className="mt-2 text-xl tracking-[-0.04em]">{note.title}</h2>
                <p className="mt-2 font-copy text-base leading-7 text-[var(--muted)]">
                  {note.description}
                </p>
              </div>
            ))}
          </div>
        </aside>
      </section>

      {/* Import swap demo */}
      <section className="section-rule mt-20 grid gap-6 lg:mt-24 lg:grid-cols-[11rem_1fr]">
        <p className="eyebrow">Import swap</p>
        <div className="space-y-5">
          <p className="copy-tone font-copy max-w-3xl text-xl leading-8">
            The integration stays familiar. Replace the import, pass a session id, keep the rest
            of your client flow.
          </p>
          <CodeSwapPanel />
        </div>
      </section>

      {/* Integrations */}
      <section className="section-rule mt-20 lg:mt-24">
        <div className="flex flex-col gap-6 md:flex-row md:items-start md:justify-between">
          <p className="eyebrow">Works with</p>
          <div className="flex flex-wrap gap-3">
            {integrations.map((name) => (
              <span key={name} className="status-pill">
                {name}
              </span>
            ))}
          </div>
        </div>
      </section>

      {/* Capabilities */}
      <section className="section-rule mt-20 grid gap-6 lg:mt-24 lg:grid-cols-[11rem_1fr]">
        <p className="eyebrow">What changes</p>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {capabilities.map((capability) => (
            <article
              key={capability.title}
              className="surface flex flex-col gap-4 p-6 transition-all duration-200 hover:-translate-y-0.5 hover:border-[var(--foreground)] md:p-8"
            >
              <h2 className="text-2xl leading-tight tracking-[-0.05em]">{capability.title}</h2>
              <p className="copy-tone font-copy text-lg leading-8">{capability.description}</p>
            </article>
          ))}
        </div>
      </section>

      {/* Plans */}
      <section className="section-rule mt-20 grid gap-6 lg:mt-24 lg:grid-cols-[11rem_1fr]">
        <p className="eyebrow">Plans</p>
        <div className="grid gap-6 md:grid-cols-3">
          {plans.map((plan) => (
            <article
              key={plan.name}
              className={`surface flex flex-col p-6 transition-all duration-300 hover:-translate-y-1 md:p-8 ${
                plan.featured ? "border-[var(--foreground)]" : "hover:border-foreground"
              }`}
            >
              <div className="flex items-center justify-between">
                <p className="eyebrow">{plan.detail}</p>
                {plan.featured && (
                  <span className="status-pill" style={{ borderColor: "var(--foreground)", color: "var(--foreground)" }}>
                    Popular
                  </span>
                )}
              </div>
              <div className="mt-4 flex items-end gap-3">
                <h2 className="text-4xl tracking-[-0.07em]">{plan.price}</h2>
                <p className="mb-1 text-sm uppercase tracking-[0.16em] text-[var(--muted)]">
                  {plan.cadence}
                </p>
              </div>
              <h3 className="mt-5 text-2xl tracking-[-0.05em]">{plan.name}</h3>
              <ul className="copy-tone mt-6 flex-grow space-y-3 font-copy text-lg leading-7">
                {plan.items.map((item) => (
                  <li key={item} className="flex gap-2">
                    <span className="text-[var(--muted)]">—</span>
                    {item}
                  </li>
                ))}
              </ul>
              <div className="mt-8 border-t border-[var(--line)] pt-4">
                <a
                  href={plan.href}
                  className="text-link"
                  {...(plan.external ? { target: "_blank", rel: "noreferrer" } : {})}
                >
                  {plan.label}
                </a>
              </div>
            </article>
          ))}
        </div>
      </section>

      {/* Footer */}
      <footer className="section-rule mt-20 flex flex-col gap-6 md:mt-24 md:flex-row md:items-end md:justify-between">
        <div className="max-w-xl space-y-3">
          <p className="eyebrow">Velarix</p>
          <p className="copy-tone font-copy text-lg leading-7">
            Minimal surface area. Correctable runtime state. Fewer places for bad assumptions to
            hide.
          </p>
        </div>
        <div className="flex flex-wrap gap-x-6 gap-y-3">
          <a href="/docs" className="text-link">
            Docs
          </a>
          <a href="/signup" className="text-link">
            Get started
          </a>
          <a
            href="https://github.com/qeinstein/velarix"
            target="_blank"
            rel="noreferrer"
            className="text-link"
          >
            GitHub
          </a>
          <span className="eyebrow">© {year}</span>
        </div>
      </footer>
    </main>
  );
}
