export type DocCodeBlock = {
  language: "python" | "typescript" | "bash" | "text";
  title: string;
  lines: string[];
};

export type DocCallout = {
  tone: "note" | "warning";
  title: string;
  body: string;
};

export type DocPanel = {
  tone: "neutral" | "logic" | "warning" | "danger" | "success";
  title: string;
  body: string;
};

export type DocSection = {
  id: string;
  title: string;
  paragraphs?: string[];
  bullets?: string[];
  codeBlocks?: DocCodeBlock[];
  callouts?: DocCallout[];
  panels?: DocPanel[];
  diagram?: "belief-graph";
};

export type DocPage = {
  slug: string[];
  groupId: string;
  navLabel: string;
  title: string;
  titleAccent?: string;
  navAccent?: boolean;
  eyebrow: string;
  description: string;
  sections: DocSection[];
};

export type DocsNavItem = {
  label: string;
  href: string;
  slug: string[];
  accent?: boolean;
};

export type DocsNavGroup = {
  id: string;
  title: string;
  items: DocsNavItem[];
};

export const DEFAULT_DOC_SLUG = ["getting-started", "introduction"] as const;

export function slugToHref(slug: readonly string[]) {
  return `/docs/${slug.join("/")}`;
}

export const docsNav: DocsNavGroup[] = [
  {
    id: "getting-started",
    title: "Getting Started",
    items: [
      {
        label: "Introduction",
        href: "/docs/getting-started/introduction",
        slug: ["getting-started", "introduction"],
      },
      {
        label: "Quick Start",
        href: "/docs/getting-started/quick-start",
        slug: ["getting-started", "quick-start"],
      },
      {
        label: "The Modern AI Stack",
        href: "/docs/getting-started/modern-ai-stack",
        slug: ["getting-started", "modern-ai-stack"],
      },
      {
        label: "Installation",
        href: "/docs/getting-started/installation",
        slug: ["getting-started", "installation"],
      },
      {
        label: "Making Your First Request",
        href: "/docs/getting-started/making-your-first-request",
        slug: ["getting-started", "making-your-first-request"],
      },
    ],
  },
  {
    id: "core-reasoning",
    title: "Core Reasoning",
    items: [
      {
        label: "Belief Graphs",
        href: "/docs/core-reasoning/belief-graphs",
        slug: ["core-reasoning", "belief-graphs"],
      },
      {
        label: "Causal Collapse",
        href: "/docs/core-reasoning/causal-collapse",
        slug: ["core-reasoning", "causal-collapse"],
        accent: true,
      },
      {
        label: "Truth Persistence",
        href: "/docs/core-reasoning/truth-persistence",
        slug: ["core-reasoning", "truth-persistence"],
      },
      {
        label: "Logic Validation",
        href: "/docs/core-reasoning/logic-validation",
        slug: ["core-reasoning", "logic-validation"],
      },
    ],
  },
  {
    id: "security",
    title: "Kernel Security (eBPF)",
    items: [
      {
        label: "Interception Logic",
        href: "/docs/security/ebpf",
        slug: ["security", "ebpf"],
      },
      {
        label: "Action Firewalls",
        href: "/docs/security/action-firewalls",
        slug: ["security", "action-firewalls"],
      },
      {
        label: "SOC2 & Compliance",
        href: "/docs/security/soc2-compliance",
        slug: ["security", "soc2-compliance"],
      },
      {
        label: "Encrypted Audit Trails",
        href: "/docs/security/encrypted-audit-trails",
        slug: ["security", "encrypted-audit-trails"],
      },
    ],
  },
  {
    id: "agentic-engineering",
    title: "Agentic Engineering",
    items: [
      {
        label: "Multi-Agent Mesh",
        href: "/docs/agentic-engineering/multi-agent-mesh",
        slug: ["agentic-engineering", "multi-agent-mesh"],
      },
      {
        label: "State Synchronization",
        href: "/docs/agentic-engineering/state-synchronization",
        slug: ["agentic-engineering", "state-synchronization"],
      },
      {
        label: "Atomic Invalidation",
        href: "/docs/agentic-engineering/atomic-invalidation",
        slug: ["agentic-engineering", "atomic-invalidation"],
      },
    ],
  },
  {
    id: "api-reference",
    title: "API Reference",
    items: [
      {
        label: "SDK Configuration",
        href: "/docs/api-reference/sdk-configuration",
        slug: ["api-reference", "sdk-configuration"],
      },
      {
        label: "Python Reference",
        href: "/docs/api-reference/python-reference",
        slug: ["api-reference", "python-reference"],
      },
      {
        label: "Node.js Reference",
        href: "/docs/api-reference/node-reference",
        slug: ["api-reference", "node-reference"],
      },
      {
        label: "Error Codes",
        href: "/docs/api-reference/error-codes",
        slug: ["api-reference", "error-codes"],
      },
    ],
  },
];

const pages: DocPage[] = [
  {
    slug: ["getting-started", "introduction"],
    groupId: "getting-started",
    navLabel: "Introduction",
    title: "Introduction",
    eyebrow: "Getting Started",
    description:
      "Velarix is an epistemic runtime for autonomous systems. It turns agent memory into a directed graph of explicit facts and justification sets, then resolves each downstream decision against the currently valid state of that graph.",
    sections: [
      {
        id: "overview",
        title: "Overview",
        paragraphs: [
          "The core engine is implemented in Go and models session state as a directed acyclic graph of facts and justifications. Root facts represent asserted premises. Derived facts depend on one or more justification sets. A derived fact remains valid only while at least one full justification path remains above the confidence threshold.",
          "This matters because model output is probabilistic while production control paths are not. Velarix separates generation from state authority. An adapter can still call a model, but the active slice, the explanation chain, and the invalidation path are resolved against the session graph rather than inferred after the fact.",
          "In practice, that means agents can carry forward memory, decisions, and provenance without treating stale context as truth. If a premise is retracted, the engine recalculates every dependent fact and removes invalid descendants from the active slice before the next turn consumes them.",
        ],
        callouts: [
          {
            tone: "note",
            title: "Implementation boundary",
            body: "The repository currently ships the API, SDK, runtime adapter, invalidation engine, impact analysis, and explanation paths. Kernel-level eBPF enforcement is not implemented in this tree.",
          },
        ],
      },
      {
        id: "runtime-architecture",
        title: "Runtime Architecture",
        paragraphs: [
          "The production pattern in this codebase is consistent across providers: fetch or derive the current belief slice, inject the Velarix protocol and tools into the model call, execute the provider request, then persist any recorded observations back into the session. The Python and TypeScript OpenAI adapters both delegate to a shared runtime for that flow.",
          "At the session layer, the SDK exposes a small set of state primitives: observe, derive, invalidate, getSlice or get_slice, explain, getImpact, revalidate, and appendHistory. These methods are the stable operating surface. They are the contract a product team should build against when integrating reasoning control into agent workflows.",
        ],
      },
      {
        id: "why-velarix",
        title: "Why Velarix Exists",
        paragraphs: [
          "Most agent stacks can tell you what the model produced. Fewer can tell you which premises were still valid when that output was generated, which dependencies the output relied on, and what actions become unsafe after a premise is withdrawn. Velarix exists to close that gap.",
          "The engine is built around deterministic state management rather than prompt-only memory. Every meaningful fact is explicit, every justification path is inspectable, and every invalidation event can be analyzed before or after it propagates.",
        ],
      },
    ],
  },
  {
    slug: ["getting-started", "quick-start"],
    groupId: "getting-started",
    navLabel: "Quick Start",
    title: "Quick Start",
    eyebrow: "Getting Started",
    description:
      "Install the SDK, create a session-aware client, and make a provider call through the shared Velarix runtime.",
    sections: [
      {
        id: "install",
        title: "Install",
        codeBlocks: [
          {
            language: "bash",
            title: "Python",
            lines: ["pip install velarix"],
          },
          {
            language: "bash",
            title: "Node.js",
            lines: ["npm install velarix-sdk"],
          },
        ],
        callouts: [
          {
            tone: "note",
            title: "Source-backed package names",
            body: "These commands come directly from sdks/python/setup.py and sdks/typescript/package.json in this repository.",
          },
        ],
      },
      {
        id: "first-session",
        title: "Create a Session",
        paragraphs: [
          "Use a stable session identifier for an operational workflow such as a claim review, support case, or agent task chain. In both SDKs, the session object is the boundary where assertions, derived facts, invalidation, slices, and explanations live.",
        ],
        codeBlocks: [
          {
            language: "python",
            title: "Python",
            lines: [
              "from velarix.client import VelarixClient",
              "",
              "client = VelarixClient(",
              '    base_url=os.environ["VELARIX_BASE_URL"],',
              '    api_key=os.environ["VELARIX_API_KEY"],',
              ")",
              'session = client.session("claims-prod")',
            ],
          },
          {
            language: "typescript",
            title: "Node.js",
            lines: [
              'import { VelarixClient } from "velarix-sdk";',
              "",
              "const client = new VelarixClient({",
              "  baseUrl: process.env.VELARIX_BASE_URL,",
              "  apiKey: process.env.VELARIX_API_KEY,",
              "});",
              'const session = client.session("claims-prod");',
            ],
          },
        ],
      },
      {
        id: "provider-call",
        title: "Run a Provider Call",
        paragraphs: [
          "The OpenAI adapters already route through the shared chat runtime. That means context injection, reasoning tool registration, and observation persistence happen inside the adapter rather than in handwritten app logic.",
        ],
        codeBlocks: [
          {
            language: "python",
            title: "Python adapter",
            lines: [
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
          {
            language: "typescript",
            title: "Node.js adapter",
            lines: [
              'import OpenAI from "openai";',
              'import { VelarixClient, VelarixOpenAI } from "velarix-sdk";',
              "",
              'const openai = new OpenAI({ apiKey: process.env.OPENAI_API_KEY });',
              "const velarix = new VelarixClient({",
              "  baseUrl: process.env.VELARIX_BASE_URL,",
              "  apiKey: process.env.VELARIX_API_KEY,",
              "});",
              "const client = new VelarixOpenAI(openai, velarix);",
              "",
              'const response = await client.chat("claims-prod", {',
              '  model: "gpt-4o",',
              '  messages: [{ role: "user", content: "Review this claim for manual escalation." }],',
              "});",
            ],
          },
        ],
      },
    ],
  },
  {
    slug: ["getting-started", "modern-ai-stack"],
    groupId: "getting-started",
    navLabel: "The Modern AI Stack",
    title: "The Modern AI Stack",
    eyebrow: "Getting Started",
    description:
      "Velarix fits between model orchestration and production control planes, where agent state needs to remain explicit, replayable, and revocable.",
    sections: [
      {
        id: "stack-boundary",
        title: "Where Velarix Sits",
        paragraphs: [
          "The model layer still generates candidates. The orchestration layer still decides which tools or providers to invoke. Velarix becomes the state authority that records the beliefs an agent relies on and determines which of them remain valid after the world changes.",
          "In this repository, that boundary is enforced through API sessions, SDK clients, provider adapters, and audit history. The result is a narrower but stronger contract: the application can ask what is currently valid, what depends on a premise, and what changed after an invalidation event.",
        ],
      },
    ],
  },
  {
    slug: ["getting-started", "installation"],
    groupId: "getting-started",
    navLabel: "Installation",
    title: "Installation",
    eyebrow: "Getting Started",
    description:
      "Use the package names and optional extras defined by the repository.",
    sections: [
      {
        id: "package-installation",
        title: "Package Installation",
        codeBlocks: [
          {
            language: "bash",
            title: "Python",
            lines: [
              "pip install velarix",
              "pip install velarix[local]",
              "pip install velarix[langchain]",
            ],
          },
          {
            language: "bash",
            title: "Node.js",
            lines: ["npm install velarix-sdk"],
          },
        ],
        callouts: [
          {
            tone: "note",
            title: "Why the extras are documented",
            body: "The Python client explicitly references velarix[local] for sidecar execution, and setup.py defines langchain, langgraph, llamaindex, and local extras.",
          },
        ],
      },
      {
        id: "environment",
        title: "Environment",
        panels: [
          {
            tone: "logic",
            title: "VELARIX_BASE_URL",
            body: "Use the API origin for your Velarix deployment or hosted environment.",
          },
          {
            tone: "neutral",
            title: "VELARIX_API_KEY",
            body: "Use a scoped key for authenticated session operations and org access.",
          },
          {
            tone: "warning",
            title: "OPENAI_API_KEY",
            body: "Required only when you are routing model calls through the OpenAI adapters.",
          },
        ],
      },
    ],
  },
  {
    slug: ["getting-started", "making-your-first-request"],
    groupId: "getting-started",
    navLabel: "Making Your First Request",
    title: "Making Your First Request",
    eyebrow: "Getting Started",
    description:
      "Your first request should bind a session, assert a fact, and run a model call through a provider adapter.",
    sections: [
      {
        id: "request-flow",
        title: "Request Flow",
        paragraphs: [
          "A minimal production request is three steps: create a session, assert or derive the facts you already know, then call the provider adapter for the next reasoning step. That keeps the model call grounded in the same state graph the rest of the system will inspect.",
        ],
        codeBlocks: [
          {
            language: "python",
            title: "Python",
            lines: [
              'session.observe("claim_submitted", payload={"claim_id": "clm_1024"})',
              "",
              "response = client.chat.completions.create(",
              '    model="gpt-4o",',
              '    messages=[{"role": "user", "content": "Review this claim for manual escalation."}],',
              ")",
            ],
          },
          {
            language: "typescript",
            title: "Node.js",
            lines: [
              'await session.observe("claim_submitted", { claimId: "clm_1024" });',
              "",
              'const response = await client.chat("claims-prod", {',
              '  model: "gpt-4o",',
              '  messages: [{ role: "user", content: "Review this claim for manual escalation." }],',
              "});",
            ],
          },
        ],
      },
    ],
  },
  {
    slug: ["core-reasoning", "belief-graphs"],
    groupId: "core-reasoning",
    navLabel: "Belief Graphs",
    title: "Belief Graphs",
    eyebrow: "Core Reasoning",
    description:
      "A session is a graph of root premises and derived facts connected through justification sets.",
    sections: [
      {
        id: "graph-model",
        title: "Graph Model",
        paragraphs: [
          "Every fact has an ID, payload, and resolved status. Root facts are asserted directly. Derived facts include one or more justification sets. Velarix uses OR-of-AND logic: a fact stays valid when at least one justification set remains fully valid.",
          "This structure is what makes later invalidation deterministic. The engine never needs to guess why a fact exists because each edge is explicit and each justification set tracks its own validity.",
        ],
        diagram: "belief-graph",
      },
    ],
  },
  {
    slug: ["core-reasoning", "causal-collapse"],
    groupId: "core-reasoning",
    navLabel: "Causal Collapse",
    title: "What is Causal Collapse?",
    navAccent: true,
    eyebrow: "Core Reasoning",
    description:
      "Causal Collapse is the invalidation path that removes downstream facts after a premise becomes invalid.",
    sections: [
      {
        id: "definition",
        title: "Definition",
        paragraphs: [
          "In the engine, root invalidation is explicit. `InvalidateRoot` only accepts root facts, marks the root as invalid, adds it to the collapsed root set, increments the mutation counter, and propagates the change through the graph.",
          "Propagation then recalculates every dependent justification set. Each justification set tracks `TargetValidParents`, `CurrentValidParents`, and a resolved confidence. If the set no longer has enough valid parents, its confidence drops to invalid. If all justification sets attached to a child fact fail, the child fact itself becomes invalid and joins the propagation queue.",
          "This is the practical meaning of Causal Collapse in Velarix: invalidation is not a metadata label. It is a graph mutation that removes unsupported facts from the active state and forces all dependents to re-resolve against current truth.",
        ],
      },
      {
        id: "dependency-invalidation",
        title: "Dependency Invalidation",
        paragraphs: [
          "The code path is visible in `core/engine.go`. Derived facts begin invalid, then each justification set is evaluated against parent statuses. During invalidation, the engine simulates the changed status, recomputes justification confidence, and pushes children back through the queue whenever a previously valid dependency becomes unsatisfied.",
          "Because validity is derived from the maximum confidence of the remaining justification sets, a fact can survive a collapse if it still has another complete support path. This is why the OR-of-AND model matters. Velarix is not just pruning descendants blindly; it is recalculating whether any alternate justification path still stands.",
        ],
        codeBlocks: [
          {
            language: "text",
            title: "Conceptual flow",
            lines: [
              "1. Invalidate a root premise.",
              "2. Recompute each dependent justification set.",
              "3. Drop any child fact with no valid support path.",
              "4. Continue until the queue is empty.",
            ],
          },
        ],
      },
      {
        id: "impact-analysis",
        title: "Impact Analysis and Counterfactuals",
        paragraphs: [
          "Velarix also exposes `GetImpact` and `ExplainReasoning`. `GetImpact` simulates a proposed invalidation and returns the impacted IDs, direct count, total count, action count, and epistemic loss. `ExplainReasoning` walks the causal chain for a fact and can attach a counterfactual narrative describing what would change if a specific premise were removed.",
          "That combination matters operationally. Before retracting a fact, you can estimate blast radius. After the collapse, you can explain exactly which premises supported a remaining decision and which facts would disappear under a counterfactual removal.",
        ],
        callouts: [
          {
            tone: "note",
            title: "Code-backed explanation",
            body: "This page is based on the current implementations of AssertFact, InvalidateRoot, GetImpact, and ExplainReasoning in core/engine.go and core/explain.go.",
          },
        ],
      },
    ],
  },
  {
    slug: ["core-reasoning", "truth-persistence"],
    groupId: "core-reasoning",
    navLabel: "Truth Persistence",
    title: "Truth Persistence",
    eyebrow: "Core Reasoning",
    description:
      "Durable storage, journals, and replay preserve state transitions beyond a single model call.",
    sections: [
      {
        id: "persistence",
        title: "Persistence Model",
        paragraphs: [
          "Velarix stores session state in BadgerDB and journals assertions, invalidations, and administrative actions. The security docs describe AES-256 encryption at rest and hash-backed export integrity.",
          "Persistence matters because invalidation is temporal. A system needs to know not only the current slice but also what changed, who changed it, and how to reconstruct the reasoning state at a specific point in time.",
        ],
      },
    ],
  },
  {
    slug: ["core-reasoning", "logic-validation"],
    groupId: "core-reasoning",
    navLabel: "Logic Validation",
    title: "Logic Validation",
    eyebrow: "Core Reasoning",
    description:
      "Validation is resolved from explicit parent state, not prompt-level confidence language.",
    sections: [
      {
        id: "validation-rules",
        title: "Validation Rules",
        paragraphs: [
          "Root facts use `manual_status`. Derived facts resolve from the confidence of their valid justification sets. A fact is treated as invalid whenever its resolved status falls below the shared confidence threshold.",
          "In other words, validity in Velarix is structural. It depends on support paths and current parent state rather than an opaque textual explanation emitted by the model.",
        ],
      },
    ],
  },
  {
    slug: ["security", "ebpf"],
    groupId: "security",
    navLabel: "Interception Logic",
    title: "Interception Logic",
    eyebrow: "Kernel Security (eBPF)",
    description:
      "The current repository ships adapter and API interception rather than a kernel-resident eBPF enforcement layer.",
    sections: [
      {
        id: "boundary",
        title: "Current Boundary",
        paragraphs: [
          "The codebase exposes interception at the provider and session layer. Python and TypeScript adapters wrap provider calls, inject the Velarix runtime protocol, and persist observations through the session API. API handlers enforce authentication, authorization, and organization boundaries for state mutation and export surfaces.",
          "There is no eBPF program or kernel module in this repository today. The term appears in product language, but the shipped enforcement surface is the application boundary: adapters, session APIs, access controls, audit history, and deterministic invalidation.",
        ],
        callouts: [
          {
            tone: "warning",
            title: "Architectural note",
            body: "Treat kernel/eBPF language in the current repository as architectural direction, not an implemented subsystem.",
          },
        ],
      },
    ],
  },
  {
    slug: ["security", "action-firewalls"],
    groupId: "security",
    navLabel: "Action Firewalls",
    title: "Action Firewalls",
    eyebrow: "Kernel Security (eBPF)",
    description:
      "Action gating in the current tree is achieved through state validation, scoped credentials, and explicit decision recording.",
    sections: [
      {
        id: "gating",
        title: "Action Gating",
        paragraphs: [
          "A production action should depend on validated session state, not solely on model text. Scoped API keys, session-level invalidation, and decision records give the application enough structure to gate actions behind current truth and verified history.",
        ],
      },
    ],
  },
  {
    slug: ["security", "soc2-compliance"],
    groupId: "security",
    navLabel: "SOC2 & Compliance",
    title: "SOC2 & Compliance",
    eyebrow: "Kernel Security (eBPF)",
    description:
      "Compliance support comes from traceability, access controls, and export integrity rather than post-hoc reconstruction.",
    sections: [
      {
        id: "compliance",
        title: "Compliance Surface",
        paragraphs: [
          "The repository documents actor attribution, access logs, retention configuration, export scope checks, and tamper-evident journal integrity. Those are the concrete compliance surfaces currently present in code and docs.",
        ],
      },
    ],
  },
  {
    slug: ["security", "encrypted-audit-trails"],
    groupId: "security",
    navLabel: "Encrypted Audit Trails",
    title: "Encrypted Audit Trails",
    eyebrow: "Kernel Security (eBPF)",
    description:
      "Audit trails are built around journals, actor attribution, integrity metadata, and encrypted persistence.",
    sections: [
      {
        id: "audit-trails",
        title: "Audit Trails",
        paragraphs: [
          "Security documentation in the repository describes encrypted BadgerDB storage, SHA-256 verified exports, actor attribution on journal entries, and restricted access to export and access-log surfaces.",
        ],
      },
    ],
  },
  {
    slug: ["agentic-engineering", "multi-agent-mesh"],
    groupId: "agentic-engineering",
    navLabel: "Multi-Agent Mesh",
    title: "Multi-Agent Mesh",
    eyebrow: "Agentic Engineering",
    description:
      "Velarix can act as a shared state authority when multiple agents need to coordinate through one truth-maintained substrate.",
    sections: [
      {
        id: "mesh",
        title: "Shared State",
        paragraphs: [
          "The repository does not ship a dedicated multi-agent coordinator, but the session API gives teams a single place to record facts, retractions, decisions, and histories that multiple workers can read against the same validity model.",
        ],
      },
    ],
  },
  {
    slug: ["agentic-engineering", "state-synchronization"],
    groupId: "agentic-engineering",
    navLabel: "State Synchronization",
    title: "State Synchronization",
    eyebrow: "Agentic Engineering",
    description:
      "Synchronization is achieved by reading from the current valid slice and writing state transitions back into the same session graph.",
    sections: [
      {
        id: "synchronization",
        title: "Synchronization",
        paragraphs: [
          "Use `getSlice` or `get_slice` to load the current truth before a turn and use observe, derive, invalidate, or revalidate to commit state transitions. That gives every worker a common, replayable state model.",
        ],
      },
    ],
  },
  {
    slug: ["agentic-engineering", "atomic-invalidation"],
    groupId: "agentic-engineering",
    navLabel: "Atomic Invalidation",
    title: "Atomic Invalidation",
    eyebrow: "Agentic Engineering",
    description:
      "Invalidation is scoped as an explicit mutation on a root premise followed by deterministic propagation.",
    sections: [
      {
        id: "atomicity",
        title: "Invalidation Semantics",
        paragraphs: [
          "The invalidation operation itself is singular and explicit. Once a root fact is invalidated, the engine recalculates dependents against the new state and the active slice updates accordingly. Applications can then inspect impact or explanations against that mutated graph.",
        ],
      },
    ],
  },
  {
    slug: ["api-reference", "sdk-configuration"],
    groupId: "api-reference",
    navLabel: "SDK Configuration",
    title: "SDK Configuration",
    eyebrow: "API Reference",
    description:
      "Both SDKs are configured around a base URL, API key, and a session-specific operating surface.",
    sections: [
      {
        id: "configuration",
        title: "Configuration",
        bullets: [
          "Python: VelarixClient(base_url, api_key, cache_ttl, max_retries, timeout_s).",
          "TypeScript: new VelarixClient({ baseUrl, apiKey, maxRetries }).",
          "Provider adapters layer on top of the session API rather than replacing it.",
        ],
      },
    ],
  },
  {
    slug: ["api-reference", "python-reference"],
    groupId: "api-reference",
    navLabel: "Python Reference",
    title: "Python Reference",
    eyebrow: "API Reference",
    description:
      "The Python SDK exposes sync and async clients plus provider and framework integrations.",
    sections: [
      {
        id: "python-surface",
        title: "Core Surface",
        bullets: [
          "VelarixClient / AsyncVelarixClient",
          "session.observe, derive, invalidate, get_slice, explain, revalidate",
          "velarix.adapters.openai.OpenAI / AsyncOpenAI",
          "VelarixChatRuntime / AsyncVelarixChatRuntime",
        ],
      },
    ],
  },
  {
    slug: ["api-reference", "node-reference"],
    groupId: "api-reference",
    navLabel: "Node.js Reference",
    title: "Node.js Reference",
    eyebrow: "API Reference",
    description:
      "The TypeScript SDK exposes a matching session surface and an OpenAI adapter built on the shared chat runtime.",
    sections: [
      {
        id: "node-surface",
        title: "Core Surface",
        bullets: [
          "VelarixClient and VelarixSession",
          "observe, derive, invalidate, getSlice, getImpact, explain, revalidate",
          "VelarixOpenAI",
          "VelarixChatRuntime utilities in src/runtime/chat.ts",
        ],
      },
    ],
  },
  {
    slug: ["api-reference", "error-codes"],
    groupId: "api-reference",
    navLabel: "Error Codes",
    title: "Error Codes",
    eyebrow: "API Reference",
    description:
      "Most SDK methods surface HTTP failures directly. Operationally, you should expect validation, permission, and retryable transport errors.",
    sections: [
      {
        id: "errors",
        title: "Operational Errors",
        panels: [
          {
            tone: "danger",
            title: "Validation Failures",
            body: "400-class responses indicate malformed facts, unknown parents, or invalid session operations.",
          },
          {
            tone: "warning",
            title: "Authorization Failures",
            body: "401 and 403 responses indicate missing credentials, invalid keys, or insufficient org scope.",
          },
          {
            tone: "logic",
            title: "Retryable Transport",
            body: "429, 502, 503, and 504 should be treated as retryable by the SDK retry path or caller policy.",
          },
          {
            tone: "neutral",
            title: "Local Runtime Startup",
            body: "Optional sidecar execution fails locally when the Velarix binary is unavailable or misconfigured.",
          },
        ],
      },
    ],
  },
];

export const docsPages = pages;

export function getDocBySlug(slug?: string[]) {
  if (!slug || slug.length === 0) {
    return pages.find((page) => page.slug.join("/") === DEFAULT_DOC_SLUG.join("/"))!;
  }

  return pages.find((page) => page.slug.join("/") === slug.join("/"));
}

export function getDocByHref(href: string) {
  return pages.find((page) => slugToHref(page.slug) === href);
}

export function getAllDocSlugs() {
  return pages.map((page) => page.slug);
}
