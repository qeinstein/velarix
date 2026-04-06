type NavSection = {
  id: string;
  label: string;
  note: string;
};

type DocsCard = {
  title: string;
  body: string;
};

type CodeStep = {
  eyebrow: string;
  title: string;
  body: string;
  code: string;
};

type TableRow = {
  name: string;
  value: string;
  note: string;
};

const navSections: NavSection[] = [
  { id: "start", label: "Start", note: "Local quickstart and workflow" },
  { id: "runtime", label: "Runtime", note: "Facts, decisions, explanations" },
  { id: "deploy", label: "Deploy", note: "Production direction and env" },
  { id: "operate", label: "Operate", note: "Health, limits, retention" },
  { id: "security", label: "Security", note: "Hardening and threat model" },
  { id: "reference", label: "Reference", note: "API, SDK, repo map" },
];

const quickstartSteps: CodeStep[] = [
  {
    eyebrow: "Step 01",
    title: "Run the API locally",
    body: "The maintained local path is Go plus Badger in dev mode. This is the fastest way to explore the approval-guardrail flow end to end.",
    code: `export VELARIX_ENV=dev
export VELARIX_API_KEY=dev-admin-key
export VELARIX_BADGER_PATH="$(mktemp -d)"

go run main.go`,
  },
  {
    eyebrow: "Step 02",
    title: "Record approval facts",
    body: "Use the Python client to open a session, assert direct observations, derive the recommendation, and persist a decision against that fact graph.",
    code: `from velarix.client import VelarixClient

client = VelarixClient(
    base_url="http://localhost:8080",
    api_key="dev-admin-key",
)

session = client.session("payment_approval_001")
session.observe("vendor_verified", {"vendor_id": "vendor-17"})
session.observe("invoice_approved", {"invoice_id": "inv-1042"})
session.observe("budget_available", {"cost_center": "ENG-01"})

session.derive(
    "decision.release_payment",
    [["vendor_verified", "invoice_approved", "budget_available"]],
    {"summary": "Payment can be released"},
)`,
  },
  {
    eyebrow: "Step 03",
    title: "Check before executing",
    body: "The product story is execution integrity. Call execute-check immediately before the side effect, then execute only if the decision is still valid.",
    code: `decision = session.create_decision(
    "payment_release",
    fact_id="decision.release_payment",
    subject_ref="inv-1042",
    target_ref="vendor-17",
    dependency_fact_ids=[
        "vendor_verified",
        "invoice_approved",
        "budget_available",
    ],
)

check = session.execute_check(decision["decision_id"])
if check["executable"]:
    session.execute_decision(decision["decision_id"])`,
  },
];

const realityCards: DocsCard[] = [
  {
    title: "What the repo ships today",
    body: "Go API, first-class decisions, session facts, invalidation, explanation endpoints, a Python SDK, a demo approval workflow, local Badger storage, and a production path centered on Postgres plus optional Redis coordination.",
  },
  {
    title: "What it does not claim",
    body: "This repo is not a finished finance-ops SaaS, not a generic memory platform, and not an audited compliance product. The docs below stay honest about current delivery versus production direction.",
  },
];

const runtimeCards: DocsCard[] = [
  {
    title: "Facts and derived facts",
    body: "A fact can be asserted directly or derived from other facts through OR-of-AND justification sets. Root observations and derived conclusions live in the same session graph.",
  },
  {
    title: "Decisions and dependency snapshots",
    body: "A decision is created from a fact and stores the dependencies that justified the recommendation. That snapshot is checked again before execution.",
  },
  {
    title: "Invalidation and pruning",
    body: "When a root fact changes, downstream conclusions can collapse if they no longer have valid support. The theory and benchmark docs describe this as O(1) pruning through dominator-style ancestry checks.",
  },
  {
    title: "Explanation and counterfactuals",
    body: "The explain surfaces do more than dump state. They expose the chain of belief, blocked dependencies, and counterfactual reasoning about what would change if a fact disappeared.",
  },
];

const workflowCards: DocsCard[] = [
  {
    title: "1. Observe",
    body: "Collect the approval facts that matter: vendor verification, invoice approval, policy sign-off, budget, access grant, or other hard prerequisites.",
  },
  {
    title: "2. Derive",
    body: "Create the derived recommendation that represents the action you want to take. This is where the graph becomes operational instead of descriptive.",
  },
  {
    title: "3. Decide",
    body: "Persist a first-class decision with subject references, targets, policy metadata, and dependency fact ids so execution is bound to a stable snapshot.",
  },
  {
    title: "4. Re-check",
    body: "Call execute-check immediately before money moves, access changes, or state mutates. If dependencies went stale, the action should stop here.",
  },
  {
    title: "5. Explain",
    body: "If a decision is blocked, inspect why-blocked, impact, explanation, history, and lineage. The repo treats operator visibility as part of the product, not an afterthought.",
  },
];

const deploymentCards: DocsCard[] = [
  {
    title: "Local development",
    body: "Badger is the local adapter. Use it for tests, demos, and fast iteration. It should not define the long-term production shape of the service.",
  },
  {
    title: "Production direction",
    body: "The target runtime is Postgres as the system of record, Redis for idempotency and rate limiting, and rebuildable in-memory engine caches instead of a permanently warm single node.",
  },
  {
    title: "Hosted console path",
    body: "The Next.js app in web/ points to the public API through NEXT_PUBLIC_VELARIX_API_URL. The private control-plane folder describes billing, provisioning, and infrastructure direction for Velarix Cloud.",
  },
];

const deploymentSteps: CodeStep[] = [
  {
    eyebrow: "Build",
    title: "Containerize the API",
    body: "The root Dockerfile currently builds the Go binary and starts it in lite mode by default. For production, run without --lite so the authenticated routes are mounted.",
    code: `docker build -t velarix-engine .
docker run --rm -p 8080:8080 \\
  -e VELARIX_ENV=prod \\
  -e VELARIX_STORE_BACKEND=postgres \\
  -e VELARIX_POSTGRES_DSN="postgres://..." \\
  -e VELARIX_REDIS_URL="redis://..." \\
  -e VELARIX_JWT_SECRET="replace-me" \\
  -e VELARIX_ENCRYPTION_KEY="replace-me" \\
  velarix-engine ./velarix`,
  },
  {
    eyebrow: "Frontend",
    title: "Point the console at the API",
    body: "The web app is a separate deployment unit. After your API is live, inject its public base URL into the Next.js frontend before building or deploying web/.",
    code: `cd web
export NEXT_PUBLIC_VELARIX_API_URL="https://api.example.com"
npm run build`,
  },
  {
    eyebrow: "Cloud",
    title: "Use the documented GCP path if you need managed infra",
    body: "The repo already includes a deployment guide centered on Cloud Run, Cloud SQL, and Memorystore, with Vercel or a second Run service for the frontend.",
    code: `gcloud run deploy velarix-api \\
  --image gcr.io/$PROJECT_ID/velarix-engine:latest \\
  --region us-central1 \\
  --set-env-vars="VELARIX_ENV=prod" \\
  --set-env-vars="VELARIX_STORE_BACKEND=postgres" \\
  --set-env-vars="VELARIX_POSTGRES_DSN=postgres://..." \\
  --set-env-vars="VELARIX_REDIS_URL=redis://..."`,
  },
];

const environmentRows: TableRow[] = [
  {
    name: "VELARIX_ENV",
    value: "dev | prod",
    note: "Controls production-only safeguards such as strict encryption requirements and secure cookies.",
  },
  {
    name: "VELARIX_STORE_BACKEND",
    value: "badger | postgres",
    note: "Selects the persistence strategy. Postgres is the intended production direction.",
  },
  {
    name: "VELARIX_POSTGRES_DSN",
    value: "postgres://...",
    note: "Required when using the Postgres-backed runtime store.",
  },
  {
    name: "VELARIX_REDIS_URL",
    value: "redis://...",
    note: "Used for the shared-store production path for rate limiting and coordination.",
  },
  {
    name: "VELARIX_JWT_SECRET",
    value: "secret string",
    note: "Required for secure console auth outside local development.",
  },
  {
    name: "VELARIX_ENCRYPTION_KEY",
    value: "32-byte key",
    note: "Required outside dev when encryption at rest is expected.",
  },
  {
    name: "VELARIX_ALLOWED_ORIGINS",
    value: "comma-separated origins",
    note: "Controls browser access for the console and other web clients.",
  },
  {
    name: "VELARIX_API_KEY",
    value: "admin or service key",
    note: "Used in local flows and service-to-service access patterns.",
  },
  {
    name: "NEXT_PUBLIC_VELARIX_API_URL",
    value: "https://api.example.com",
    note: "Build-time API target for the Next.js console in web/.",
  },
];

const operateCards: DocsCard[] = [
  {
    title: "Health endpoints",
    body: "GET /health is the basic liveness check. GET /health/full exposes deeper health information for admin and operator use.",
  },
  {
    title: "Backpressure and retries",
    body: "Write saturation returns 503 with Retry-After: 1 and X-Velarix-Backpressure: 1. The repo expects clients to retry safely with idempotency keys.",
  },
  {
    title: "Rate limiting",
    body: "Local and shared-store paths exist today, but the long-term production shape should be Redis-backed rather than tied to local process assumptions.",
  },
  {
    title: "Retention and recovery",
    body: "Badger backup and restore exist for the local adapter. The production story should center on Postgres backup strategy, Redis recovery, and artifact handling for exports and snapshots.",
  },
];

const errorRows: TableRow[] = [
  {
    name: "409 Conflict",
    value: "Stale decision execution",
    note: "The key approval-guardrail failure mode. Inspect reason_codes, blocked_by, and why-blocked before retrying.",
  },
  {
    name: "429 Too Many Requests",
    value: "Rate limit exceeded",
    note: "Honor Retry-After and resend with the same idempotency key where appropriate.",
  },
  {
    name: "503 Service Unavailable",
    value: "Per-org write backpressure",
    note: "Signals burst protection. Retry with the same idempotency key once pressure clears.",
  },
  {
    name: "401 / 403",
    value: "Auth or scope failure",
    note: "Usually caused by missing API keys, invalid JWTs, cross-org access, or insufficient route scope.",
  },
  {
    name: "400 / 404",
    value: "Bad payload or missing fact",
    note: "Common when justification sets are malformed, facts are absent, or a decision points to missing dependencies.",
  },
];

const securityCards: DocsCard[] = [
  {
    title: "What exists today",
    body: "API key auth, JWT user auth, org-scoped request handling, append-only history with verification hashes, scope-aware route checks, and optional Badger encryption at rest.",
  },
  {
    title: "Primary threats",
    body: "Cross-tenant access, stolen service credentials, stale decision execution, history tampering, and denial of service from bursty writers are the core threats modeled in the repo.",
  },
  {
    title: "Practical guidance",
    body: "If the workflow can move money, change access, or create audit exposure, require a fresh execute-check immediately before the business action.",
  },
  {
    title: "Honest gaps",
    body: "Password reset is not a finished production delivery flow, shared-store defaults still need to replace local assumptions, and export hashes are not compliance evidence.",
  },
];

const endpointGroups = [
  {
    title: "Sessions and facts",
    rows: [
      ["POST /v1/s/{session_id}/facts", "Assert root or derived facts into a session."],
      ["POST /v1/s/{session_id}/facts/{id}/invalidate", "Invalidate a root fact and collapse stale descendants."],
      ["GET /v1/s/{session_id}/facts/{id}", "Fetch a fact directly."],
      ["GET /v1/s/{session_id}/graph", "Return the session graph used by the dashboard explorer."],
      ["GET /v1/s/{session_id}/history", "Return append-only session history."],
      ["GET /v1/s/{session_id}/slice", "Render the current valid slice as JSON or markdown."],
    ],
  },
  {
    title: "Decisions and execution",
    rows: [
      ["POST /v1/s/{session_id}/decisions", "Create a first-class decision from a fact or dependency set."],
      ["POST /v1/s/{session_id}/decisions/{decision_id}/recompute", "Refresh dependency state and recompute status."],
      ["POST /v1/s/{session_id}/decisions/{decision_id}/execute-check", "Ask whether the decision is still executable."],
      ["POST /v1/s/{session_id}/decisions/{decision_id}/execute", "Attempt execution and block if dependencies are stale."],
      ["GET /v1/s/{session_id}/decisions/{decision_id}/why-blocked", "Explain the blocking facts for a stale decision."],
      ["GET /v1/s/{session_id}/decisions/{decision_id}/lineage", "Inspect upstream and downstream decision lineage."],
    ],
  },
  {
    title: "Explanation, export, and org views",
    rows: [
      ["GET /v1/s/{session_id}/explain", "Return reasoning and counterfactual explanation for a fact."],
      ["GET /v1/s/{session_id}/facts/{id}/impact", "Estimate downstream impact if the fact changes."],
      ["GET /v1/s/{session_id}/export", "Export session state directly."],
      ["POST /v1/s/{session_id}/export-jobs", "Create an export job for download workflows."],
      ["GET /v1/org/decisions", "List org-level decision records."],
      ["GET /v1/org/usage", "Return usage counters for the current org."],
    ],
  },
  {
    title: "Auth, keys, and console services",
    rows: [
      ["POST /v1/auth/register", "Create a console account."],
      ["POST /v1/auth/login", "Issue a JWT-backed console session."],
      ["POST /v1/auth/reset-request", "Begin password reset flow."],
      ["GET /v1/keys", "List generated service keys."],
      ["POST /v1/keys/generate", "Create a new API key."],
      ["DELETE /v1/keys/{key}", "Revoke a key."],
    ],
  },
];

const sdkRows: TableRow[] = [
  {
    name: "VelarixClient / session()",
    value: "Python SDK",
    note: "Primary entry point for session-scoped operations in sdks/python/velarix/client.py.",
  },
  {
    name: "observe / derive / invalidate",
    value: "Fact lifecycle",
    note: "Assert direct observations, derived facts, and invalidations against the session graph.",
  },
  {
    name: "create_decision / execute_check / execute_decision",
    value: "Execution integrity",
    note: "The core approval-guardrail path recommended by docs/INTEGRATION_GUIDE.md.",
  },
  {
    name: "adapters.openai.OpenAI",
    value: "Model-provider surface",
    note: "OpenAI-compatible adapter for feeding observations and explanations through the same runtime.",
  },
  {
    name: "mcp_server.py",
    value: "MCP integration",
    note: "Exposes assert_fact, get_fact, explain_reasoning, and a session context resource for MCP clients.",
  },
];

const repoRows: TableRow[] = [
  {
    name: "main.go",
    value: "Runtime bootstrap",
    note: "Selects lite mode, storage backend, encryption checks, and server startup behavior.",
  },
  {
    name: "api/server.go",
    value: "Public HTTP surface",
    note: "Mounts the /v1 routes for sessions, org views, auth, billing, and decision workflows.",
  },
  {
    name: "api/decision_contracts.go",
    value: "Product wedge",
    note: "Canonical decision and approval-integrity flow according to docs/README.md.",
  },
  {
    name: "demo/approval_integrity.py",
    value: "Maintained demo",
    note: "Shows decision creation, stale invalidation, blocked execution, and explanation.",
  },
  {
    name: "tests/e2e_test.go",
    value: "End-to-end behavior",
    note: "Useful anchor when validating actual repo behavior beyond the docs and demo.",
  },
];

function Section({
  id,
  label,
  title,
  summary,
  children,
}: {
  id: string;
  label: string;
  title: string;
  summary: string;
  children: React.ReactNode;
}) {
  return (
    <section id={id} className="section-rule scroll-mt-28">
      <div className="grid gap-6 xl:grid-cols-[12rem_1fr]">
        <div className="space-y-2">
          <p className="eyebrow">{label}</p>
          <h2 className="text-3xl tracking-[-0.06em]">{title}</h2>
        </div>
        <div className="space-y-8">
          <p className="copy-tone font-copy max-w-3xl text-xl leading-8">{summary}</p>
          {children}
        </div>
      </div>
    </section>
  );
}

function DataTable({
  columns,
  rows,
}: {
  columns: [string, string, string];
  rows: TableRow[];
}) {
  return (
    <div className="overflow-x-auto border border-[var(--line)]">
      <table className="docs-table">
        <thead>
          <tr>
            <th>{columns[0]}</th>
            <th>{columns[1]}</th>
            <th>{columns[2]}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.name}>
              <td>
                <code>{row.name}</code>
              </td>
              <td>{row.value}</td>
              <td>{row.note}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default function Docs() {
  return (
    <main className="pb-24 pt-10 md:pt-16">
      <header className="grid gap-10 border-b border-[var(--line)] pb-10 lg:grid-cols-[minmax(0,1.3fr)_18rem]">
        <div className="space-y-7">
          <p className="eyebrow">Documentation</p>
          <h1 className="font-display max-w-5xl text-[clamp(3.4rem,9vw,6.6rem)] leading-[0.9] tracking-[-0.08em]">
            Production-minded docs for an approval-guardrail runtime.
          </h1>
          <p className="copy-tone font-copy max-w-3xl text-[1.16rem] leading-8">
            This page is built from the repo as it exists today: Go API, Python SDK, decision
            workflows, deployment notes, operational constraints, and the production direction
            described by the codebase itself.
          </p>
          <div className="flex flex-wrap gap-3">
            {navSections.map((section) => (
              <a key={section.id} href={`#${section.id}`} className="button-ghost">
                {section.label}
              </a>
            ))}
          </div>
        </div>

        <div className="grid gap-px border border-[var(--line)] bg-[var(--line)]">
          <div className="surface p-5">
            <p className="field-label">Runtime</p>
            <p className="mt-3 text-2xl tracking-[-0.05em]">Go API</p>
            <p className="copy-tone mt-2 font-copy text-base leading-7">
              Facts, invalidation, decisions, explanations, exports, auth, and org-level views.
            </p>
          </div>
          <div className="surface p-5">
            <p className="field-label">SDK</p>
            <p className="mt-3 text-2xl tracking-[-0.05em]">Python client</p>
            <p className="copy-tone mt-2 font-copy text-base leading-7">
              Session-first integration surface plus OpenAI adapter and MCP server support.
            </p>
          </div>
          <div className="surface p-5">
            <p className="field-label">Direction</p>
            <p className="mt-3 text-2xl tracking-[-0.05em]">Postgres + Redis</p>
            <p className="copy-tone mt-2 font-copy text-base leading-7">
              Local Badger stays for development. Shared-state production should not depend on one warm node.
            </p>
          </div>
        </div>
      </header>

      <div className="mt-14 grid gap-10 xl:grid-cols-[15rem_minmax(0,1fr)]">
        <aside className="hidden xl:block">
          <div className="sticky top-24 space-y-3">
            <p className="eyebrow">Docs map</p>
            {navSections.map((section) => (
              <a key={section.id} href={`#${section.id}`} className="docs-sidebar-link">
                <span className="font-mono text-[0.72rem] uppercase tracking-[0.16em]">
                  {section.label}
                </span>
                <span className="font-copy text-sm leading-6">{section.note}</span>
              </a>
            ))}
          </div>
        </aside>

        <div className="space-y-16">
          <Section
            id="start"
            label="Start"
            title="Use Velarix close to execution"
            summary="The strongest integration path in this repo is not generic memory. It is approval integrity: capture facts, derive a recommendation, create a decision, and re-check before the action fires."
          >
            <div className="grid gap-4 lg:grid-cols-2">
              {realityCards.map((card) => (
                <article key={card.title} className="surface p-6">
                  <h3 className="text-2xl tracking-[-0.05em]">{card.title}</h3>
                  <p className="copy-tone mt-3 font-copy text-lg leading-8">{card.body}</p>
                </article>
              ))}
            </div>

            <div className="space-y-6">
              {quickstartSteps.map((step) => (
                <article key={step.title} className="grid gap-4 border-t border-[var(--line)] pt-6 lg:grid-cols-[12rem_1fr] lg:gap-8">
                  <div className="space-y-2">
                    <p className="eyebrow">{step.eyebrow}</p>
                    <h3 className="text-2xl tracking-[-0.05em]">{step.title}</h3>
                  </div>
                  <div className="space-y-4">
                    <p className="copy-tone font-copy text-lg leading-8">{step.body}</p>
                    <div className="dark-surface code-panel p-5 md:p-6">
                      <pre>{step.code}</pre>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          </Section>

          <Section
            id="runtime"
            label="Runtime"
            title="Facts, decisions, and explanations"
            summary="The repo models approval state as a fact graph. Decisions bind an action to that graph. Explanation surfaces are there so stale execution can be understood and debugged instead of merely denied."
          >
            <div className="grid gap-4 md:grid-cols-2">
              {runtimeCards.map((card) => (
                <article key={card.title} className="surface p-6">
                  <h3 className="text-2xl tracking-[-0.05em]">{card.title}</h3>
                  <p className="copy-tone mt-3 font-copy text-lg leading-8">{card.body}</p>
                </article>
              ))}
            </div>

            <div className="space-y-4 border-t border-[var(--line)] pt-6">
              <p className="eyebrow">Execution flow</p>
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                {workflowCards.map((card) => (
                  <article key={card.title} className="surface p-5">
                    <h3 className="text-xl tracking-[-0.04em]">{card.title}</h3>
                    <p className="copy-tone mt-3 font-copy text-base leading-7">{card.body}</p>
                  </article>
                ))}
              </div>
            </div>
          </Section>

          <Section
            id="deploy"
            label="Deploy"
            title="Treat local and production paths differently"
            summary="The codebase is explicit about this: Badger is the local adapter, not the product architecture. Production-like deployments should use Postgres, Redis, secure auth material, and a separately deployed frontend."
          >
            <div className="grid gap-4 md:grid-cols-3">
              {deploymentCards.map((card) => (
                <article key={card.title} className="surface p-6">
                  <h3 className="text-2xl tracking-[-0.05em]">{card.title}</h3>
                  <p className="copy-tone mt-3 font-copy text-lg leading-8">{card.body}</p>
                </article>
              ))}
            </div>

            <div className="space-y-6">
              {deploymentSteps.map((step) => (
                <article key={step.title} className="grid gap-4 border-t border-[var(--line)] pt-6 lg:grid-cols-[12rem_1fr] lg:gap-8">
                  <div className="space-y-2">
                    <p className="eyebrow">{step.eyebrow}</p>
                    <h3 className="text-2xl tracking-[-0.05em]">{step.title}</h3>
                  </div>
                  <div className="space-y-4">
                    <p className="copy-tone font-copy text-lg leading-8">{step.body}</p>
                    <div className="dark-surface code-panel p-5 md:p-6">
                      <pre>{step.code}</pre>
                    </div>
                  </div>
                </article>
              ))}
            </div>

            <div className="space-y-4 border-t border-[var(--line)] pt-6">
              <p className="eyebrow">Required configuration</p>
              <DataTable columns={["Variable", "Value", "Why it matters"]} rows={environmentRows} />
            </div>
          </Section>

          <Section
            id="operate"
            label="Operate"
            title="Observe health, pressure, and failure modes"
            summary="The repo already documents the operational contract. Health endpoints, retry behavior, backpressure headers, and retention rules matter as much as the fact model once the service sits in front of a real execution system."
          >
            <div className="grid gap-4 md:grid-cols-2">
              {operateCards.map((card) => (
                <article key={card.title} className="surface p-6">
                  <h3 className="text-2xl tracking-[-0.05em]">{card.title}</h3>
                  <p className="copy-tone mt-3 font-copy text-lg leading-8">{card.body}</p>
                </article>
              ))}
            </div>

            <div className="space-y-4 border-t border-[var(--line)] pt-6">
              <p className="eyebrow">Operator-facing errors</p>
              <DataTable columns={["Status", "Meaning", "Expected response"]} rows={errorRows} />
            </div>
          </Section>

          <Section
            id="security"
            label="Security"
            title="Secure the approval path, not just the API surface"
            summary="Velarix is valuable when it prevents stale, high-impact actions from executing. The repo's security posture is therefore tied to provenance, org boundaries, key handling, and fresh execute-checks."
          >
            <div className="grid gap-4 md:grid-cols-2">
              {securityCards.map((card) => (
                <article key={card.title} className="surface p-6">
                  <h3 className="text-2xl tracking-[-0.05em]">{card.title}</h3>
                  <p className="copy-tone mt-3 font-copy text-lg leading-8">{card.body}</p>
                </article>
              ))}
            </div>

            <div className="soft-surface p-6">
              <p className="eyebrow">Hardening rule</p>
              <p className="copy-tone mt-3 font-copy text-xl leading-8">
                If a workflow can move money, change access, or create audit exposure, require a
                fresh execute-check immediately before the final action.
              </p>
            </div>
          </Section>

          <Section
            id="reference"
            label="Reference"
            title="Map the API surface back to the repo"
            summary="The current codebase exposes more routes than the short marketing docs suggest. The groups below reflect the actual server surface and the SDK entry points you would care about after deployment."
          >
            <div className="space-y-6">
              {endpointGroups.map((group) => (
                <article key={group.title} className="border border-[var(--line)]">
                  <div className="surface border-b border-[var(--line)] px-5 py-4">
                    <h3 className="text-2xl tracking-[-0.05em]">{group.title}</h3>
                  </div>
                  <div className="overflow-x-auto">
                    <table className="docs-table">
                      <thead>
                        <tr>
                          <th>Route</th>
                          <th>Description</th>
                          <th>Source</th>
                        </tr>
                      </thead>
                      <tbody>
                        {group.rows.map(([route, description]) => (
                          <tr key={route}>
                            <td>
                              <code>{route}</code>
                            </td>
                            <td>{description}</td>
                            <td>
                              <code>api/server.go</code>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </article>
              ))}
            </div>

            <div className="space-y-4 border-t border-[var(--line)] pt-6">
              <p className="eyebrow">SDK and integration surfaces</p>
              <DataTable columns={["Surface", "Layer", "Why it matters"]} rows={sdkRows} />
            </div>

            <div className="space-y-4 border-t border-[var(--line)] pt-6">
              <p className="eyebrow">Canonical files to read</p>
              <DataTable columns={["File", "Role", "What to inspect"]} rows={repoRows} />
            </div>

            <div className="flex flex-wrap gap-3 border-t border-[var(--line)] pt-6">
              <a href="https://github.com/qeinstein/velarix" target="_blank" rel="noreferrer" className="button-solid">
                View repository
              </a>
              <a href="/signup" className="button-ghost">
                Open console
              </a>
            </div>
          </Section>
        </div>
      </div>
    </main>
  );
}
