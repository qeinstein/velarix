import React, { useState } from 'react';
import { Book, ChevronRight, Menu, X, ArrowLeft, Terminal, Shield, Cpu, Zap, HelpCircle } from 'lucide-react';

interface DocsProps {
  onBack: () => void;
}

const sections = [
  { 
    id: 'intro', 
    title: 'Introduction', 
    icon: Book,
    content: (
      <div className="space-y-6">
        <h1 className="text-4xl font-bold text-white mb-4">What is Velarix?</h1>
        <p className="text-lg text-slate-400 leading-relaxed">
          Velarix is a high-performance epistemic orchestration layer designed to solve the #1 problem in autonomous AI: <strong>Stale Context Hallucinations.</strong>
        </p>
        <p className="text-slate-400 leading-relaxed">
          Unlike flat vector databases that store facts as append-only logs, Velarix treats agent memory as a dynamic dependency graph of justified beliefs. When a foundational premise changes, Velarix instantly collapses the entire tree of dependent assumptions, ensuring the agent's context window remains mathematically consistent.
        </p>
        <div className="p-6 bg-indigo-600/10 border border-indigo-500/20 rounded-2xl">
          <h3 className="font-bold text-white mb-2 flex items-center gap-2"><Zap className="w-4 h-4 text-indigo-400" /> The Core Innovation</h3>
          <p className="text-sm text-slate-400">
            Velarix uses <strong>Dominator-based Pruning</strong> (compiler theory) to logically sever stale reasoning chains in O(1) time on the write-path.
          </p>
        </div>
      </div>
    )
  },
  { 
    id: 'get-started', 
    title: 'Getting Started', 
    icon: Zap,
    content: (
      <div className="space-y-6">
        <h1 className="text-4xl font-bold text-white mb-4">Quickstart</h1>
        <p className="text-slate-400">Integrate Velarix into your existing AI application in under 10 minutes.</p>
        
        <h3 className="text-xl font-bold text-white mt-8">1. Get your API Key</h3>
        <p className="text-sm text-slate-400">Visit <a href="#" className="text-indigo-400 hover:underline">velarix.dev/keys</a> to generate your unique access token.</p>

        <h3 className="text-xl font-bold text-white mt-8">2. Install the SDK</h3>
        <div className="bg-slate-900 rounded-xl p-4 font-mono text-sm border border-slate-800">
          <span className="text-slate-500">$</span> pip install velarix
        </div>

        <h3 className="text-xl font-bold text-white mt-8">3. Swap Your Import</h3>
        <p className="text-sm text-slate-400 mb-4">Velarix is a drop-in replacement for the OpenAI client.</p>
        <div className="bg-slate-900 rounded-xl p-4 font-mono text-sm border border-slate-800 overflow-x-auto">
          <pre className="text-slate-300">
{`# from openai import OpenAI
from velarix.adapters.openai import OpenAI

client = OpenAI(
    api_key="your-openai-key",
    velarix_api_key="vx_...", 
    velarix_session_id="user_123"
)

# Injection and extraction are now automatic
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "I prefer dark mode."}]
)`}
          </pre>
        </div>
      </div>
    )
  },
  { 
    id: 'api-reference', 
    title: 'API Reference', 
    icon: Terminal,
    content: (
      <div className="space-y-8">
        <h1 className="text-4xl font-bold text-white mb-4">API Endpoints</h1>
        
        <div className="space-y-6">
          <div className="p-6 bg-slate-900 border border-slate-800 rounded-2xl">
            <div className="flex items-center gap-3 mb-4">
              <span className="px-2 py-1 bg-blue-900/30 text-blue-400 text-[10px] font-bold rounded uppercase tracking-widest">GET</span>
              <code className="text-white font-bold text-sm">/s/&#123;session_id&#125;/slice</code>
            </div>
            <p className="text-sm text-slate-400 mb-4">Generates a prompt-ready snapshot of valid context.</p>
            <div className="bg-slate-950 p-4 rounded-xl border border-slate-800">
              <p className="text-[10px] font-bold text-slate-500 uppercase mb-2">Query Params</p>
              <ul className="text-xs space-y-1 text-slate-400 font-mono">
                <li>format: "json" | "markdown"</li>
                <li>max_facts: number (default 50)</li>
              </ul>
            </div>
          </div>

          <div className="p-6 bg-slate-900 border border-slate-800 rounded-2xl">
            <div className="flex items-center gap-3 mb-4">
              <span className="px-2 py-1 bg-emerald-900/30 text-emerald-400 text-[10px] font-bold rounded uppercase tracking-widest">POST</span>
              <code className="text-white font-bold text-sm">/s/&#123;session_id&#125;/facts</code>
            </div>
            <p className="text-sm text-slate-400 mb-4">Assert a new fact or derive a belief.</p>
          </div>
        </div>
      </div>
    )
  },
  { 
    id: 'schema-enforcement', 
    title: 'Schema Enforcement', 
    icon: Shield,
    content: (
      <div className="space-y-6">
        <h1 className="text-4xl font-bold text-white mb-4">Schema Layer</h1>
        <p className="text-slate-400 leading-relaxed">
          Prevent "Prompt Poisoning" by enforcing structure on agent observations. Velarix supports full JSON Schema validation in two distinct modes.
        </p>
        
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-8">
          <div className="p-6 bg-indigo-600/5 border border-indigo-500/20 rounded-2xl">
            <h3 className="font-bold text-white mb-2">Strict Mode</h3>
            <p className="text-xs text-slate-400">Outright rejects facts that violate the schema. Returns 400 Bad Request to the agent, forcing a correction loop.</p>
          </div>
          <div className="p-6 bg-amber-600/5 border border-amber-500/20 rounded-2xl">
            <h3 className="font-bold text-white mb-2">Warn Mode</h3>
            <p className="text-xs text-slate-400">Accepts the fact but flags it with validation errors in the audit trail. Useful for debugging soft hallucinations.</p>
          </div>
        </div>
      </div>
    )
  },
  { 
    id: 'faq', 
    title: 'FAQ', 
    icon: HelpCircle,
    content: (
      <div className="space-y-8">
        <h1 className="text-4xl font-bold text-white mb-4">FAQ</h1>
        <div className="space-y-6">
          <div>
            <h3 className="text-white font-bold mb-2">Is this a Vector Database?</h3>
            <p className="text-sm text-slate-400">No. Vector DBs handle similarity search. Velarix handles <strong>logical consistency</strong>. Most developers use them together: Vector DB for the long-tail of knowledge, Velarix for the agent's current plan and session state.</p>
          </div>
          <div>
            <h3 className="text-white font-bold mb-2">Does this run on my infra?</h3>
            <p className="text-sm text-slate-400">No. Velarix is a managed infrastructure layer. You point our SDK at our hosted orchestrator URL.</p>
          </div>
        </div>
      </div>
    )
  }
];

export function Docs({ onBack }: DocsProps) {
  const [activeSection, setActiveSection] = useState('intro');
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div className="flex h-screen bg-slate-950 text-slate-200 font-sans overflow-hidden">
      {/* Mobile Toggle */}
      <button 
        onClick={() => setSidebarOpen(!sidebarOpen)}
        className="fixed bottom-6 right-6 z-50 p-4 bg-indigo-600 rounded-full shadow-lg md:hidden"
      >
        {sidebarOpen ? <X /> : <Menu />}
      </button>

      {/* Docs Sidebar */}
      <aside className={`
        fixed inset-0 z-40 bg-slate-950 border-r border-slate-800 transition-transform md:relative md:translate-x-0 md:w-72
        ${sidebarOpen ? 'translate-x-0' : '-translate-x-full'}
      `}>
        <div className="p-6 border-b border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 bg-indigo-600 rounded-md flex items-center justify-center">
              <Book className="w-3.5 h-3.5 text-white" />
            </div>
            <span className="font-bold tracking-tight text-white uppercase text-xs tracking-widest">Documentation</span>
          </div>
          <button onClick={onBack} className="text-slate-500 hover:text-white transition-colors"><ArrowLeft className="w-4 h-4" /></button>
        </div>

        <nav className="p-4 space-y-1 overflow-y-auto h-full pb-24">
          {sections.map(s => (
            <button
              key={s.id}
              onClick={() => { setActiveSection(s.id); setSidebarOpen(false); }}
              className={`w-full flex items-center gap-3 px-4 py-2.5 rounded-xl text-sm font-medium transition-all ${
                activeSection === s.id 
                  ? 'bg-indigo-600/10 text-white border border-indigo-500/20' 
                  : 'text-slate-500 hover:text-slate-300 border border-transparent'
              }`}
            >
              <s.icon className={`w-4 h-4 ${activeSection === s.id ? 'text-indigo-400' : ''}`} />
              {s.title}
              {activeSection === s.id && <ChevronRight className="w-3.5 h-3.5 ml-auto text-indigo-500" />}
            </button>
          ))}
        </nav>
      </aside>

      {/* Docs Content */}
      <main className="flex-1 overflow-y-auto bg-slate-950/50 relative">
        <div className="max-w-3xl mx-auto p-8 md:p-16 py-24">
          <div className="animate-in fade-in slide-in-from-bottom-2 duration-500">
            {sections.find(s => s.id === activeSection)?.content}
          </div>
          
          <div className="mt-24 pt-8 border-t border-slate-800 flex justify-between items-center opacity-50">
            <div className="flex items-center gap-2">
              <Cpu className="w-4 h-4" />
              <span className="text-[10px] font-bold uppercase tracking-widest">Velarix Epistemic Core</span>
            </div>
            <span className="text-[10px] font-mono">v0.1.0-alpha</span>
          </div>
        </div>
      </main>
    </div>
  );
}
