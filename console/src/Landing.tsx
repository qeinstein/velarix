import React from 'react';
import { 
  Database, 
  Shield, 
  Cpu, 
  Zap, 
  GitBranch, 
  Layers, 
  ArrowRight, 
  Terminal, 
  ExternalLink,
  Code,
  Activity
} from 'lucide-react';

interface LandingProps {
  onGetStarted: () => void;
  onLogin: () => void;
  onDocs: () => void;
}

export function Landing({ onGetStarted, onLogin, onDocs }: LandingProps) {
  return (
    <div className="min-h-screen bg-[#020617] text-slate-200 font-sans selection:bg-indigo-500/30">
      {/* Nav */}
      <nav className="fixed top-0 w-full z-50 border-b border-white/5 bg-[#020617]/80 backdrop-blur-md">
        <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 bg-indigo-600 rounded-lg flex items-center justify-center shadow-lg shadow-indigo-500/20">
              <Database className="w-5 h-5 text-white" />
            </div>
            <span className="font-bold tracking-tight text-white text-lg">Velarix</span>
          </div>
          <div className="flex items-center gap-8">
            <button onClick={onDocs} className="text-sm font-medium text-slate-400 hover:text-white transition-colors">Documentation</button>
            <button onClick={onLogin} className="text-sm font-medium text-slate-400 hover:text-white transition-colors">Login</button>
            <button 
              onClick={onGetStarted}
              className="bg-white text-black px-4 py-2 rounded-full text-sm font-bold hover:bg-slate-200 transition-all flex items-center gap-2"
            >
              Get API Key <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="relative pt-40 pb-32 px-6 overflow-hidden">
        {/* Background Glow */}
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-full max-w-4xl h-[500px] bg-indigo-600/10 blur-[120px] rounded-full pointer-events-none" />
        
        <div className="max-w-4xl mx-auto text-center relative z-10">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 text-xs font-bold uppercase tracking-widest mb-8">
            <Zap className="w-3 h-3 text-indigo-400" /> v0.1.0-alpha available now
          </div>
          <h1 className="text-6xl md:text-7xl font-bold tracking-tighter text-white mb-8 leading-[1.1]">
            The Epistemic State Layer <br />
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-indigo-400 via-white to-slate-400">
              for AI Agents
            </span>
          </h1>
          <p className="text-xl text-slate-400 leading-relaxed mb-12 max-w-2xl mx-auto">
            Velarix is a high-performance logic firewall for AI agents. It ensures that when a foundational premise changes, all dependent reasoning instantly collapses—giving you a zero-hallucination orchestration layer that slots into your stack in minutes.
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
            <button 
              onClick={onGetStarted}
              className="w-full sm:w-auto px-8 py-4 bg-indigo-600 hover:bg-indigo-500 text-white rounded-2xl font-bold transition-all shadow-xl shadow-indigo-500/20 flex items-center justify-center gap-2 text-lg"
            >
              Get started for free <ArrowRight className="w-5 h-5" />
            </button>
            <button 
              onClick={onDocs}
              className="w-full sm:w-auto px-8 py-4 bg-slate-900 hover:bg-slate-800 text-slate-200 border border-slate-800 rounded-2xl font-bold transition-all flex items-center justify-center gap-2 text-lg"
            >
              Read the docs
            </button>
          </div>
        </div>
      </section>

      {/* Code Snippet */}
      <section className="py-24 px-6 relative">
        <div className="max-w-5xl mx-auto">
          <div className="bg-[#0a0f1e] border border-white/5 rounded-[2rem] p-1 overflow-hidden shadow-2xl">
            <div className="bg-[#020617] rounded-[1.9rem] p-8 md:p-12 relative overflow-hidden">
              <div className="flex items-center gap-3 mb-10">
                <div className="w-3 h-3 rounded-full bg-red-500/20 border border-red-500/40" />
                <div className="w-3 h-3 rounded-full bg-amber-500/20 border border-amber-500/40" />
                <div className="w-3 h-3 rounded-full bg-emerald-500/20 border border-emerald-500/40" />
                <div className="ml-4 text-xs font-mono text-slate-500 uppercase tracking-widest">A 4-line drop-in for deterministic reasoning</div>
              </div>
              
              <div className="grid grid-cols-1 md:grid-cols-2 gap-12 items-center">
                <div className="space-y-6">
                  <h3 className="text-3xl font-bold text-white tracking-tight">Zero Refactoring Required.</h3>
                  <p className="text-slate-400 leading-relaxed">
                    Velarix is a drop-in replacement for the OpenAI client. It intercepts your LLM calls to automatically inject context and extract facts back into your reasoning sessions.
                  </p>
                  <ul className="space-y-4">
                    {[
                      { icon: Shield, text: "Strict JSON Schema enforcement" },
                      { icon: GitBranch, text: "O(1) causal dependency pruning" },
                      { icon: Layers, text: "Multi-tenant context isolation" }
                    ].map((item, i) => (
                      <li key={i} className="flex items-center gap-3 text-sm font-semibold text-slate-300">
                        <item.icon className="w-4 h-4 text-indigo-500" />
                        {item.text}
                      </li>
                    ))}
                  </ul>
                </div>
                
                <div className="relative">
                  <div className="absolute -inset-4 bg-indigo-500/10 blur-3xl rounded-full" />
                  <div className="relative bg-[#0f172a]/50 border border-white/10 rounded-2xl p-6 font-mono text-sm leading-relaxed shadow-inner">
                    <div className="flex items-center gap-2 text-slate-500 mb-4 border-b border-white/5 pb-4">
                      <Terminal className="w-4 h-4" /> <span className="text-[10px] font-bold uppercase tracking-widest">main.py</span>
                    </div>
                    <pre className="text-indigo-300">
{`# from openai import OpenAI
from velarix.adapters.openai import OpenAI

client = OpenAI(
    velarix_api_key="vx_...", 
    velarix_session_id="user_123"
)

# That's it.
# Context is now managed.`}
                    </pre>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Features Grid */}
      <section className="py-32 px-6">
        <div className="max-w-7xl mx-auto">
          <div className="text-center mb-20">
            <h2 className="text-[10px] font-bold text-indigo-500 uppercase tracking-[0.3em] mb-4">Core Infrastructure</h2>
            <h3 className="text-4xl md:text-5xl font-bold text-white tracking-tight">Built for Production-Grade Agents</h3>
          </div>
          
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <FeatureCard 
              icon={GitBranch}
              title="Causal Consistency"
              description="Mathematical guarantee that your agent's context window never contains contradictory assumptions."
            />
            <FeatureCard 
              icon={Layers}
              title="Multi-Tenant Isolation"
              description="Reasoning sessions are physically isolated per user, ensuring zero context leakage between conversations."
            />
            <FeatureCard 
              icon={Shield}
              title="Schema Enforcement"
              description="Reject malformed agent outputs in real-time before they poison your reasoning graph."
            />
            <FeatureCard 
              icon={Terminal}
              title="Context Slicing API"
              description="Dynamically generate prompt-ready snapshots of the active truth in Markdown or JSON."
            />
            <FeatureCard 
              icon={Cpu}
              title="O(1) Complexity"
              description="Compiler-grade Dominator Tree pruning collapses stale thoughts instantly, regardless of chain depth."
            />
            <FeatureCard 
              icon={Activity}
              title="Neural Graph Visualizer"
              description="A production-ready observability dashboard to debug and audit agent reasoning in real-time."
            />
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="py-20 px-6 border-t border-white/5">
        <div className="max-w-7xl mx-auto flex flex-col md:flex-row justify-between items-center gap-8">
          <div className="flex items-center gap-2.5">
            <Database className="w-5 h-5 text-indigo-500" />
            <span className="font-bold tracking-tight text-white uppercase text-sm tracking-widest">Velarix Epistemic Core</span>
          </div>
          <div className="flex items-center gap-12 text-sm text-slate-500 font-medium">
            <button onClick={onDocs} className="hover:text-white transition-colors">GitHub</button>
            <button onClick={onDocs} className="hover:text-white transition-colors">Twitter</button>
            <button onClick={onDocs} className="hover:text-white transition-colors">Discord</button>
          </div>
          <div className="text-[10px] font-mono text-slate-600 uppercase tracking-widest">
            © 2026 Velarix Inc. // v0.1.0-alpha
          </div>
        </div>
      </footer>
    </div>
  );
}

function FeatureCard({ icon: Icon, title, description }: { icon: any, title: string, description: string }) {
  return (
    <div className="group p-8 bg-white/[0.02] border border-white/5 rounded-[2rem] hover:bg-white/[0.04] hover:border-white/10 transition-all duration-500">
      <div className="w-12 h-12 bg-indigo-600/10 rounded-2xl flex items-center justify-center mb-6 group-hover:scale-110 transition-transform duration-500">
        <Icon className="w-6 h-6 text-indigo-500" />
      </div>
      <h4 className="text-xl font-bold text-white mb-3 tracking-tight">{title}</h4>
      <p className="text-slate-400 text-sm leading-relaxed">{description}</p>
    </div>
  );
}
