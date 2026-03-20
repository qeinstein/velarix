import { useMemo, useState } from 'react';
import {
  ArrowLeft,
  BookOpen,
  ChevronRight,
  Cpu,
  HelpCircle,
  Shield,
  Terminal,
  Zap,
} from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

interface DocsProps {
  onBack: () => void;
}

type SectionId = 'intro' | 'quickstart' | 'api' | 'schema' | 'faq';

const sections: {
  id: SectionId;
  title: string;
  icon: typeof BookOpen;
  summary: string;
}[] = [
  { id: 'intro', title: 'Introduction', icon: BookOpen, summary: 'What Velarix is actually for.' },
  { id: 'quickstart', title: 'Quickstart', icon: Zap, summary: 'From account to first session.' },
  { id: 'api', title: 'API surface', icon: Terminal, summary: 'The endpoints the console relies on.' },
  { id: 'schema', title: 'Schema policy', icon: Shield, summary: 'Strict vs warn behavior.' },
  { id: 'faq', title: 'FAQ', icon: HelpCircle, summary: 'Scope, positioning, and caveats.' },
];

export function Docs({ onBack }: DocsProps) {
  const [activeSection, setActiveSection] = useState<SectionId>('intro');

  const content = useMemo(() => {
    switch (activeSection) {
      case 'intro':
        return (
          <motion.div key="intro" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.3 }}>
            <Header
              eyebrow="Product thesis"
              title="Velarix is a reasoning integrity layer, not a generic memory bucket."
              description="The strongest message in this product is the causal model: facts have support, support can be revoked, and the context shown to the agent should reflect that reality immediately."
            />
            <InfoCard title="Core proposition" description="Traditional vector memories do similarity well but logical invalidation poorly. Velarix keeps an explicit justification graph so stale downstream beliefs can be collapsed instead of silently lingering in prompt context." />
            <GridCard
              items={[
                'The Go kernel owns reasoning state, support propagation, and invalidation.',
                'The console is the observability wedge: graph view, journal replay, impact, and blame.',
                'SDK adapters are the delivery mechanism, not the product moat on their own.',
              ]}
            />
          </motion.div>
        );
      case 'quickstart':
        return (
          <motion.div key="quickstart" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.3 }}>
            <Header
              eyebrow="Getting started"
              title="Use the backend routes the console is actually built around."
              description="The console now authenticates against the backend and provisions a real API key, so the quickstart should match that reality instead of an imagined hosted flow."
            />
            <CodeBlock
              title="Python adapter"
              code={`from velarix.adapters.openai import OpenAI

client = OpenAI(
    api_key="sk-...",
    velarix_api_key="vx_...",
    velarix_session_id="research_task"
)

client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Use React 18."}]
)`}
            />
            <GridCard
              items={[
                'Create an account or log in through the console auth screen.',
                'Use the generated API key for session reads and writes.',
                'Open the visualizer to inspect graph state, warnings, and journal history.',
              ]}
            />
          </motion.div>
        );
      case 'api':
        return (
          <motion.div key="api" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.3 }}>
            <Header
              eyebrow="API surface"
              title="The console depends on a smaller set of routes than the marketing copy implies."
              description="These are the endpoints you need to care about if you are evaluating the current implementation rather than the aspirational product story."
            />
            <Endpoint method="GET" path="/sessions" description="Lists in-memory sessions visible to the authenticated org." />
            <Endpoint method="GET" path="/s/{session_id}/facts" description="Fetches facts for the active session, optionally filtered to valid facts only." />
            <Endpoint method="GET" path="/s/{session_id}/history" description="Returns the journal used by the replay timeline and activity surfaces." />
            <Endpoint method="GET" path="/s/{session_id}/facts/{id}/impact" description="Returns the current impact report with blast radius metrics." />
            <Endpoint method="GET" path="/s/{session_id}/facts/{id}/why" description="Returns the explanation tree used to highlight provenance." />
          </motion.div>
        );
      case 'schema':
        return (
          <motion.div key="schema" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.3 }}>
            <Header
              eyebrow="Validation"
              title="Schema mode is the first real policy lever in the product."
              description="This matters because it is one of the few places where the product can decisively prevent low-quality state from entering the graph."
            />
            <div className="grid gap-4 md:grid-cols-2">
              <ModeCard
                title="Strict mode"
                tone="strict"
                description="Rejects invalid writes immediately. Best when you want the caller to fix output structure before the fact is persisted."
              />
              <ModeCard
                title="Warn mode"
                tone="warn"
                description="Accepts the write but annotates the issue so the operator can inspect it later in the console."
              />
            </div>
          </motion.div>
        );
      case 'faq':
        return (
          <motion.div key="faq" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.3 }}>
            <Header
              eyebrow="FAQ"
              title="The blunt answers matter more than the polished ones."
              description="A product positioned for enterprise users should be explicit about scope, current implementation limits, and where the console is still immature."
            />
            <FaqItem question="Is Velarix a vector database?" answer="No. It is a reasoning state layer. You would pair it with another retrieval or storage system rather than expect it to replace every memory primitive in your stack." />
            <FaqItem question="Does the current console fully match the docs?" answer="No. The project ambition is larger than the present implementation in several places, especially around polish, completeness, and consistency of behavior across the app." />
            <FaqItem question="What is the strongest implemented differentiator right now?" answer="The explicit causal and impact model. When it works, that is the part that actually feels different from generic agent tooling." />
          </motion.div>
        );
      default:
        return null;
    }
  }, [activeSection]);

  return (
    <div className="page-backdrop flex min-h-screen text-slate-100">
      <aside className="surface-panel-strong hidden w-80 shrink-0 border-r border-white/8 p-6 lg:flex lg:flex-col">
        <motion.button 
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
          onClick={onBack} 
          className="button-secondary mb-6 px-4 py-3 text-sm font-semibold shadow-lg shadow-black/20"
        >
          <ArrowLeft className="h-4 w-4" /> Back
        </motion.button>
        <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Documentation</div>
        <div className="font-display mt-2 text-3xl font-semibold text-white">Operator manual</div>
        <p className="mt-4 text-sm leading-7 text-slate-400">
          This is a product-facing summary of the implementation that exists today, not an idealized future deck.
        </p>
        <nav className="mt-8 space-y-2 relative">
          {sections.map((section) => {
            const active = section.id === activeSection;
            return (
              <button
                key={section.id}
                onClick={() => setActiveSection(section.id)}
                className={`w-full rounded-[1.4rem] px-4 py-4 text-left transition-colors relative group border ${active ? 'border-sky-300/20 text-white' : 'border-white/8 bg-black/10 hover:border-sky-300/16 hover:bg-white/[0.03] text-slate-400'}`}
              >
                {active && (
                  <motion.div layoutId="docs-active-nav" className="absolute inset-0 bg-sky-400/10 rounded-[1.4rem] -z-10 shadow-inner border border-sky-400/20" transition={{ type: 'spring', bounce: 0.2, duration: 0.5 }} />
                )}
                <div className="flex items-center gap-3 relative z-10">
                  <section.icon className={`h-4 w-4 transition-colors ${active ? 'text-sky-300' : 'text-slate-500 group-hover:text-sky-400'}`} />
                  <div className={`font-medium transition-colors ${active ? 'text-white' : 'text-slate-200'}`}>{section.title}</div>
                  {active && <ChevronRight className="ml-auto h-4 w-4 text-sky-300" />}
                </div>
                <p className={`mt-3 text-sm leading-7 relative z-10 transition-colors ${active ? 'text-sky-100/70' : 'text-slate-400'}`}>{section.summary}</p>
              </button>
            );
          })}
        </nav>
      </aside>

      <main className="flex-1 overflow-y-auto px-6 py-8 lg:px-10">
        <div className="mx-auto max-w-4xl space-y-6">
          <motion.button 
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
            onClick={onBack} 
            className="button-secondary px-4 py-3 text-sm font-semibold lg:hidden shadow-lg shadow-black/20"
          >
            <ArrowLeft className="h-4 w-4" /> Back
          </motion.button>

          <div className="surface-panel rounded-[2rem] p-6 lg:hidden">
            <div className="mb-4 text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Sections</div>
            <div className="grid gap-3 sm:grid-cols-2">
              {sections.map((section) => (
                <button
                  key={section.id}
                  onClick={() => setActiveSection(section.id)}
                  className={`rounded-[1.4rem] border px-4 py-4 text-left transition-colors relative ${section.id === activeSection ? 'border-sky-300/20 bg-sky-400/10 text-white' : 'border-white/8 bg-black/10 text-slate-300'}`}
                >
                  <div className="font-medium">{section.title}</div>
                  <p className="mt-2 text-sm leading-7 opacity-70">{section.summary}</p>
                </button>
              ))}
            </div>
          </div>

          <section className="surface-panel-strong rounded-[2rem] p-8 md:p-10 relative overflow-hidden">
            <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(14,165,233,0.05),transparent_60%)] pointer-events-none" />
            <div className="relative z-10">
              <AnimatePresence mode="wait">
                {content}
              </AnimatePresence>
            </div>
          </section>

          <div className="flex items-center gap-3 border-t border-white/8 pt-4 text-xs uppercase tracking-[0.18em] text-slate-500 font-bold">
            <Zap className="h-3 w-3" /> Velarix 0.1.0
          </div>
        </div>
      </main>
    </div>
  );
}

function Header({ eyebrow, title, description }: { eyebrow: string; title: string; description: string }) {
  return (
    <div className="mb-10">
      <div className="section-kicker">
        <BookOpen className="h-3.5 w-3.5" /> {eyebrow}
      </div>
      <h2 className="font-display mt-6 text-3xl font-bold tracking-tight text-white md:text-4xl">{title}</h2>
      <p className="mt-4 max-w-2xl text-lg leading-8 text-slate-300">{description}</p>
    </div>
  );
}

function InfoCard({ title, description }: { title: string; description: string }) {
  return (
    <motion.div 
      whileHover={{ y: -4, scale: 1.01 }}
      className="mb-8 rounded-[1.6rem] border border-sky-400/20 bg-sky-400/5 p-6 shadow-inner hover:shadow-lg hover:shadow-sky-500/10 transition-all"
    >
      <div className="flex items-center gap-3">
        <Cpu className="h-5 w-5 text-sky-400" />
        <div className="font-display text-xl font-semibold text-white">{title}</div>
      </div>
      <p className="mt-3 text-sm leading-7 text-sky-100/80">{description}</p>
    </motion.div>
  );
}

function GridCard({ items }: { items: string[] }) {
  return (
    <div className="grid gap-4 sm:grid-cols-3">
      {items.map((item, i) => (
        <motion.div 
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: i * 0.1 }}
          whileHover={{ y: -2 }}
          key={i} 
          className="rounded-[1.4rem] border border-white/8 bg-black/10 p-5 text-sm leading-7 text-slate-300 hover:bg-white/[0.03] transition-colors"
        >
          {item}
        </motion.div>
      ))}
    </div>
  );
}

function CodeBlock({ title, code }: { title: string; code: string }) {
  return (
    <div className="mb-8 overflow-hidden rounded-[1.6rem] border border-white/8 bg-[#050a14] shadow-inner">
      <div className="flex items-center gap-3 border-b border-white/8 bg-white/[0.02] px-5 py-3">
        <Terminal className="h-4 w-4 text-slate-500" />
        <div className="text-[0.65rem] uppercase tracking-[0.2em] text-slate-400 font-bold">{title}</div>
      </div>
      <pre className="overflow-x-auto p-5 text-[0.8rem] leading-relaxed text-sky-200 font-mono">
        {code}
      </pre>
    </div>
  );
}

function Endpoint({ method, path, description }: { method: string; path: string; description: string }) {
  return (
    <motion.div 
      whileHover={{ scale: 1.01 }}
      className="mb-4 flex flex-col gap-4 rounded-[1.4rem] border border-white/8 bg-black/10 p-5 hover:bg-white/[0.03] transition-colors md:flex-row md:items-center"
    >
      <div className="flex shrink-0 items-center gap-3">
        <span className={`rounded-full px-2.5 py-1 text-[0.6rem] uppercase tracking-[0.16em] font-bold ${method === 'GET' ? 'bg-sky-400/10 text-sky-300' : 'bg-emerald-400/10 text-emerald-300'}`}>
          {method}
        </span>
        <code className="text-sm font-bold text-white tracking-wide">{path}</code>
      </div>
      <p className="text-sm leading-7 text-slate-400">{description}</p>
    </motion.div>
  );
}

function ModeCard({ title, description, tone }: { title: string; description: string; tone: 'strict' | 'warn' }) {
  return (
    <motion.div 
      whileHover={{ y: -4 }}
      className={`rounded-[1.6rem] border p-6 transition-colors shadow-sm hover:shadow-lg ${tone === 'strict' ? 'border-emerald-400/20 bg-emerald-400/5 hover:border-emerald-400/40 hover:shadow-emerald-500/10' : 'border-amber-300/20 bg-amber-300/5 hover:border-amber-300/40 hover:shadow-amber-500/10'}`}
    >
      <div className="flex items-center gap-3">
        <div className={`h-2 w-2 rounded-full ${tone === 'strict' ? 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.8)]' : 'bg-amber-400 shadow-[0_0_8px_rgba(251,191,36,0.8)]'}`} />
        <div className="font-display text-xl font-semibold text-white">{title}</div>
      </div>
      <p className={`mt-3 text-sm leading-7 ${tone === 'strict' ? 'text-emerald-100/70' : 'text-amber-100/70'}`}>{description}</p>
    </motion.div>
  );
}

function FaqItem({ question, answer }: { question: string; answer: string }) {
  return (
    <motion.div 
      whileHover={{ x: 4 }}
      className="mb-6 rounded-[1.6rem] border border-white/8 bg-black/10 p-6 hover:border-white/20 transition-colors"
    >
      <div className="font-display text-lg font-semibold text-white">{question}</div>
      <p className="mt-3 text-sm leading-7 text-slate-400">{answer}</p>
    </motion.div>
  );
}
