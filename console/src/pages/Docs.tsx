import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Link, useLocation } from 'react-router-dom';
import { 
  BookOpen, 
  Layers, 
  ShieldCheck, 
  Box, 
  Zap, 
  Shield, 
  Search,
  ChevronRight,
  Code2,
  Activity,
  History,
  Network,
  Scale,
  MessageSquare,
  CheckCircle,
  Brain,
  Filter,
  ShieldAlert,
  Menu,
  X,
  ChevronDown
} from 'lucide-react';
import LetterGlitch from '../components/reactbits/LetterGlitch';

const DOCS_NAV = [
  { id: 'overview', label: 'Protocol Overview', icon: BookOpen },
  { id: 'concepts', label: 'Epistemic Concepts', icon: Layers },
  { id: 'one-line-swap', label: 'The One-Line Swap', icon: Zap },
  { id: 'sdk-usage', label: 'SDK Usage Guide', icon: Code2 },
  { id: 'truth-filtering', label: 'Truth Filtering & RAG', icon: Filter },
  { id: 'audit-logic', label: 'Audit & Provenance', icon: ShieldCheck },
  { id: 'api-reference', label: 'API Reference', icon: Box },
];

export default function Docs() {
  const [activeDocTab, setActiveDocTab] = useState('overview');
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const location = useLocation();

  // Close menus on route or tab change
  useEffect(() => {
    setIsMobileMenuOpen(false);
    setIsSidebarOpen(false);
    window.scrollTo(0, 0);
  }, [location.pathname, activeDocTab]);

  const renderContent = () => {
    switch (activeDocTab) {
      case 'overview':
        return (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-10">
            <div className="space-y-4">
              <div className="inline-flex items-center px-3 py-1 rounded-full bg-v-accent/10 border border-v-accent/20 text-[10px] font-bold text-v-accent uppercase tracking-[0.3em] mb-4">
                Global Production Cluster
              </div>
              <h2 className="text-3xl md:text-5xl font-display font-bold text-white tracking-tight leading-[1.1]">
                The Logic Layer for <br/><span className="text-v-accent">Reliable AI Agents.</span>
              </h2>
              <p className="text-lg md:text-xl text-v-text-muted leading-relaxed max-w-3xl">
                Velarix provides the deterministic "conscience" for autonomous systems. 
                Instead of flat, probabilistic memory, Velarix maintains a <span className="text-white">Justified Belief Graph</span> that 
                automatically enforces consistency across clinical, legal, and high-stakes reasoning tasks.
              </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 md:gap-8 mt-12">
              <div className="glass-panel p-8 md:p-10 rounded-[2rem] border-white/5 bg-white/[0.02] relative group">
                <div className="absolute top-0 right-0 p-6 opacity-20 group-hover:opacity-40 transition-opacity">
                  <Scale className="w-10 h-10 md:w-12 md:h-12 text-v-accent" />
                </div>
                <h3 className="text-xl font-semibold text-white mb-4">State Integrity</h3>
                <p className="text-sm text-v-text-muted leading-relaxed">Prevent "Causal Collapse." When a patient revokes consent or a lab is corrected, all dependent downstream reasoning is purged instantly and deterministically.</p>
              </div>
              <div className="glass-panel p-8 md:p-10 rounded-[2rem] border-white/5 bg-white/[0.02] relative group">
                <div className="absolute top-0 right-0 p-6 opacity-20 group-hover:opacity-40 transition-opacity">
                  <History className="w-10 h-10 md:w-12 md:h-12 text-v-success" />
                </div>
                <h3 className="text-xl font-semibold text-white mb-4">Traceable Provenance</h3>
                <p className="text-sm text-v-text-muted leading-relaxed">Every fact asserted by an LLM or Human is tagged with a cryptographic ID, timestamp, and model metadata for SOC2/HIPAA compliance.</p>
              </div>
            </div>
          </motion.div>
        );

      case 'concepts':
        return (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-12">
            <h2 className="text-3xl font-display font-bold text-white">Core Epistemic Concepts</h2>
            <div className="space-y-10">
              <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
                {[
                  { title: 'Root Facts', desc: 'Direct observations or immutable premises (e.g., "Patient is 45 years old").', color: 'v-accent' },
                  { title: 'Derived Beliefs', desc: 'Inferences made by agents that depend on other facts (e.g., "Patient is eligible for study").', color: 'v-success' },
                  { title: 'Justifications', desc: 'The "Because" links. A DAG of reasons that support why a belief is held.', color: 'white' },
                ].map((concept, i) => (
                  <div key={i} className="space-y-4">
                    <div className={`text-[10px] font-bold text-${concept.color} uppercase tracking-widest`}>0{i+1} — {concept.title}</div>
                    <p className="text-sm text-v-text-muted leading-relaxed">{concept.desc}</p>
                  </div>
                ))}
              </div>

              <div className="bg-v-bg-card/50 border border-white/10 rounded-3xl p-8 md:p-10 relative overflow-hidden">
                <div className="absolute top-0 right-0 w-64 h-64 bg-v-accent/5 blur-[100px] -mr-32 -mt-32"></div>
                <h3 className="text-xl font-display font-semibold text-white mb-6 flex items-center">
                  <Activity className="w-5 h-5 mr-3 text-v-accent" /> Dominator Tree Invalidation
                </h3>
                <p className="text-v-text-muted leading-relaxed mb-8">
                  Velarix doesn't just delete data. It traverses the dominator tree of your reasoning. 
                  If <span className="text-white">Fact A</span> is the only justification for <span className="text-white">Fact B</span>, 
                  invalidating <span className="text-white">A</span> will automatically collapse <span className="text-white">B</span>. 
                  This ensures your LLM never hallucinates based on retracted data.
                </p>
                <div className="flex items-center space-x-4 bg-black/40 p-4 rounded-xl border border-white/5 w-fit">
                  <div className="w-3 h-3 rounded-full bg-v-error animate-pulse"></div>
                  <span className="text-xs font-mono text-v-text-muted">Premise Retracted</span>
                  <ChevronRight className="w-4 h-4 text-white/20" />
                  <span className="text-xs font-mono text-white/40 line-through">Dependent Beliefs</span>
                </div>
              </div>
            </div>
          </motion.div>
        );

      case 'one-line-swap':
        return (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-8">
            <h2 className="text-3xl font-display font-bold text-white">The One-Line Swap</h2>
            <p className="text-v-text-muted text-lg">The most common integration pattern. Upgrade any OpenAI-compatible agent to use epistemic memory by changing the client import.</p>
            
            <div className="glass-panel rounded-3xl overflow-hidden border-v-accent/30 shadow-[0_0_50px_rgba(6,182,212,0.1)]">
              <div className="px-6 md:px-8 py-4 bg-v-accent/5 border-b border-v-accent/10 flex justify-between items-center">
                <span className="text-[10px] font-bold text-v-accent uppercase tracking-[0.2em]">Python Integration</span>
                <div className="flex space-x-1.5">
                  <div className="w-2 h-2 rounded-full bg-v-accent/40"></div>
                  <div className="w-2 h-2 rounded-full bg-v-accent/40"></div>
                </div>
              </div>
              <div className="p-6 md:p-10 bg-[#080808] font-mono text-xs md:text-sm leading-relaxed text-zinc-300 overflow-x-auto">
                <div className="text-v-text-muted mb-4"># Drop-in replacement for standard OpenAI client</div>
                <span className="text-v-success font-bold">from velarix.adapters.openai import OpenAI</span><br/><br/>
                <span className="text-v-text-muted"># Connect to the production cluster with a session context</span><br/>
                client = OpenAI(<br/>
                &nbsp;&nbsp;api_key=<span className="text-v-accent">"sk-..."</span>,<br/>
                &nbsp;&nbsp;velarix_session_id=<span className="text-v-accent">"patient_encounter_101"</span><br/>
                )<br/><br/>
                <span className="text-v-text-muted"># Standard OpenAI SDK usage</span><br/>
                response = client.chat.completions.create(<br/>
                &nbsp;&nbsp;model=<span className="text-v-accent">"gpt-4o"</span>,<br/>
                &nbsp;&nbsp;messages=[&#123;<span className="text-v-accent">"role"</span>: <span className="text-v-accent">"user"</span>, <span className="text-v-accent">"content"</span>: <span className="text-v-accent">"Check for medication conflicts."</span>&#125;]<br/>
                )<br/><br/>
                <span className="text-v-text-muted"># Velarix handles the reasoning graph and context injection automatically.</span>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mt-8">
              <div className="p-6 rounded-2xl bg-white/[0.02] border border-white/5">
                <h4 className="text-white font-semibold mb-2">Automated Extraction</h4>
                <p className="text-xs text-v-text-muted leading-relaxed">The adapter injects a hidden protocol that forces the LLM to use the `record_observation` tool for new insights.</p>
              </div>
              <div className="p-6 rounded-2xl bg-white/[0.02] border border-white/5">
                <h4 className="text-white font-semibold mb-2">Protocol Injection</h4>
                <p className="text-xs text-v-text-muted leading-relaxed">Existing valid facts are automatically prepended to the system prompt to maintain context parity.</p>
              </div>
            </div>
          </motion.div>
        );

      case 'sdk-usage':
        return (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-8">
            <h2 className="text-3xl font-display font-bold text-white">Manual Fact Management</h2>
            <p className="text-v-text-muted">For total control over reasoning chains, use the core SDK methods to build and break causal links.</p>
            
            <div className="space-y-10">
              <div className="space-y-4">
                <h3 className="text-lg font-medium text-white flex items-center"><Search className="w-5 h-4 mr-2 text-v-accent" /> 1. Assertion & Derivation</h3>
                <div className="glass-panel rounded-2xl overflow-hidden">
                  <div className="p-6 md:p-8 bg-black/60 font-mono text-xs md:text-sm text-zinc-300 leading-relaxed overflow-x-auto">
                    <span className="text-v-text-muted"># Assert a root premise</span><br/>
                    session.observe(<span className="text-v-accent">"hipaa_consent"</span>, payload=&#123;<span className="text-v-accent">"status"</span>: <span className="text-v-accent">"signed"</span>&#125;)<br/><br/>
                    <span className="text-v-text-muted"># Explicitly derive a conclusion based on premises</span><br/>
                    session.derive(<br/>
                    &nbsp;&nbsp;<span className="text-v-accent">"process_phi"</span>, <br/>
                    &nbsp;&nbsp;justifications=[[<span className="text-v-accent">"hipaa_consent"</span>]], <br/>
                    &nbsp;&nbsp;payload=&#123;<span className="text-v-accent">"allowed"</span>: <span className="text-v-success">True</span>&#125;<br/>
                    )
                  </div>
                </div>
              </div>

              <div className="space-y-4">
                <h3 className="text-lg font-medium text-white flex items-center"><Zap className="w-5 h-4 mr-2 text-v-accent" /> 2. Causal Retraction</h3>
                <div className="glass-panel rounded-2xl overflow-hidden">
                  <div className="p-6 md:p-8 bg-black/60 font-mono text-xs md:text-sm text-zinc-300 leading-relaxed overflow-x-auto">
                    <span className="text-v-text-muted"># Invalidate the root: "process_phi" becomes invalid instantly</span><br/>
                    session.invalidate(<span className="text-v-accent">"hipaa_consent"</span>)<br/><br/>
                    <span className="text-v-text-muted"># Verify status</span><br/>
                    status = session.get_fact(<span className="text-v-accent">"process_phi"</span>).status <span className="text-v-text-muted"># Returns 0.0 (Invalid)</span>
                  </div>
                </div>
              </div>
            </div>
          </motion.div>
        );

      case 'truth-filtering':
        return (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-8">
            <h2 className="text-3xl font-display font-bold text-white">Truth Filtering & Epistemic RAG</h2>
            <p className="text-v-text-muted text-lg leading-relaxed">
              Standard RAG pipelines suffer from "Context Corruption" where stale or retracted information persists in the prompt. 
              Velarix acts as an <span className="text-white">Active Filter</span> that only permits logically sound context to reach the LLM.
            </p>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 md:gap-8 my-10">
              <div className="p-8 rounded-3xl bg-white/[0.02] border border-white/5 relative group">
                <div className="absolute top-0 right-0 p-6 opacity-10">
                  <Brain className="w-10 h-10 text-v-accent" />
                </div>
                <h4 className="text-white font-semibold mb-4">The Challenge</h4>
                <p className="text-sm text-v-text-muted leading-relaxed">Vector databases don't understand causality. If a fact is retracted, the vector remains, leading the agent to hallucinate based on "ghost" data.</p>
              </div>
              <div className="p-8 rounded-3xl bg-v-accent/5 border border-v-accent/20 relative group">
                <div className="absolute top-0 right-0 p-6 opacity-20">
                  <Shield className="w-10 h-10 text-v-accent" />
                </div>
                <h4 className="text-white font-semibold mb-4">The Solution</h4>
                <p className="text-sm text-v-text-muted leading-relaxed">Velarix's <code className="text-white">get_slice()</code> method only returns facts with a <span className="text-v-accent">ResolvedStatus &ge; 0.6</span>, ensuring deterministic reasoning integrity.</p>
              </div>
            </div>

            <div className="space-y-6">
              <h3 className="text-xl font-display font-semibold text-white">Implementation Pattern</h3>
              <div className="glass-panel rounded-2xl overflow-hidden shadow-2xl">
                <div className="px-6 py-3 bg-white/5 border-b border-white/10 flex justify-between items-center">
                  <span className="text-[10px] font-mono text-zinc-500">Python / Epistemic RAG</span>
                  <span className="text-[10px] text-v-success uppercase tracking-widest font-bold whitespace-nowrap">Recommended</span>
                </div>
                <div className="p-6 md:p-8 bg-[#080808] font-mono text-xs md:text-sm leading-relaxed text-zinc-300 overflow-x-auto">
                  <span className="text-v-text-muted"># 1. Fetch only the logically valid 'Truth Slice'</span><br/>
                  context = session.get_slice(format=<span className="text-v-accent">"markdown"</span>, max_facts=20)<br/><br/>
                  <span className="text-v-text-muted"># 2. Inject into the prompt boundary</span><br/>
                  prompt = f<span className="text-v-accent">"""</span><br/>
                  <span className="text-v-accent">System: You are an agent with verified memory. </span><br/>
                  <span className="text-v-accent">Use only these currently valid facts for your reasoning:</span><br/>
                  <span className="text-v-accent">&#123;context&#125;</span><br/>
                  <span className="text-v-accent">---</span><br/>
                  <span className="text-v-accent">User: &#123;query&#125;</span><br/>
                  <span className="text-v-accent">"""</span><br/><br/>
                  <span className="text-v-text-muted"># 3. If a premise is retracted, get_slice() updates instantly.</span>
                </div>
              </div>
            </div>

            <div className="flex items-center space-x-4 p-6 rounded-2xl bg-v-error/5 border border-v-error/20">
              <ShieldAlert className="w-6 h-6 text-v-error shrink-0" />
              <p className="text-xs text-v-text-muted">
                <strong className="text-white">Important:</strong> The 0.6 confidence threshold is the production standard. 
                Any fact that falls below this (due to retraction or justification loss) is automatically omitted from the slice.
              </p>
            </div>
          </motion.div>
        );

      case 'audit-logic':
        return (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-8">
            <h2 className="text-3xl font-display font-bold text-white">Security & Audit Compliance</h2>
            <p className="text-v-text-muted">Velarix ensures every clinical decision is auditable and attributed to a verified actor.</p>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {[
                { title: 'Actor Attribution', desc: 'Every fact includes metadata for the API Key or User ID that asserted it.', icon: Search },
                { title: 'Immutable Journal', desc: 'State changes are recorded in a write-only transactional journal.', icon: History },
                { title: 'Tenant Isolation', desc: 'Strict OrgID partitioning ensures sessions never leak across organizations.', icon: ShieldCheck },
                { title: 'SOC2 Ready', desc: 'Generate PDF/CSV audit reports with SHA-256 verification hashes.', icon: Shield },
              ].map((item, i) => (
                <div key={i} className="p-8 rounded-3xl bg-white/[0.02] border border-white/5 hover:border-v-accent/30 transition-all">
                  <item.icon className="w-6 h-6 text-v-accent mb-4" />
                  <h4 className="text-white font-semibold mb-2">{item.title}</h4>
                  <p className="text-sm text-v-text-muted leading-relaxed">{item.desc}</p>
                </div>
              ))}
            </div>

            <div className="bg-gradient-to-br from-v-accent/10 to-transparent border border-v-accent/20 rounded-3xl p-6 md:p-8">
              <h3 className="text-xl font-display font-semibold text-white mb-4">Generate Compliance Export</h3>
              <code className="text-xs md:text-sm text-v-accent block bg-black/50 p-4 rounded-lg font-mono overflow-x-auto">
                GET /v1/s/encounter_001/export?format=pdf&verify=true
              </code>
            </div>
          </motion.div>
        );

      case 'api-reference':
        return (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="space-y-8">
            <h2 className="text-3xl font-display font-bold text-white">Production API Reference</h2>
            <p className="text-v-text-muted">Base Cluster URL: <code className="text-v-accent">https://api.velarix.ai/v1</code></p>
            
            <div className="space-y-10">
              <section>
                <h4 className="text-[10px] font-bold text-v-text-muted uppercase tracking-[0.3em] mb-6 border-b border-white/10 pb-2">Session Operations</h4>
                <div className="space-y-4">
                  {[
                    { method: 'POST', path: '/s/{id}/facts', desc: 'Assert a new root or derived fact.' },
                    { method: 'GET', path: '/s/{id}/slice', desc: 'Fetch the truth slice for prompt injection.' },
                    { method: 'GET', path: '/s/{id}/facts/{fid}/why', desc: 'Retrieve the justification tree (explanation).' },
                    { method: 'POST', path: '/s/{id}/facts/{fid}/invalidate', desc: 'Retract a premise and trigger collapse.' },
                  ].map((api, i) => (
                    <div key={i} className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 group bg-white/[0.01] p-3 md:p-4 rounded-xl hover:bg-white/[0.03] transition-all border border-transparent hover:border-white/5">
                      <span className={`w-16 text-[10px] font-bold py-1.5 rounded-lg text-center shrink-0 ${api.method === 'POST' ? 'bg-v-accent/20 text-v-accent' : 'bg-v-success/20 text-v-success'}`}>
                        {api.method}
                      </span>
                      <code className="text-xs md:text-sm text-white font-mono flex-1 break-all">{api.path}</code>
                      <span className="text-[11px] text-v-text-muted transition-opacity whitespace-normal md:whitespace-nowrap">— {api.desc}</span>
                    </div>
                  ))}
                </div>
              </section>

              <div className="p-6 rounded-2xl bg-v-accent/5 border border-v-accent/20 flex flex-col sm:flex-row items-center justify-between gap-4 text-center sm:text-left">
                <div>
                  <h5 className="text-sm font-semibold text-white">Full OpenAPI Specification</h5>
                  <p className="text-xs text-v-text-muted">Download the complete Swagger artifact for automated client generation.</p>
                </div>
                <button className="w-full sm:w-auto px-6 py-2.5 bg-v-accent text-v-bg text-xs font-bold rounded-lg hover:bg-v-accent/80 transition-all uppercase tracking-widest">
                  Download Spec
                </button>
              </div>
            </div>
          </motion.div>
        );

      default:
        return null;
    }
  };

  return (
    <div className="min-h-screen bg-v-bg relative overflow-x-hidden flex flex-col font-sans text-v-text selection:bg-v-accent/40">
      {/* Background Effect */}
      <div className="absolute inset-0 z-0 opacity-10 pointer-events-none mix-blend-screen">
        <LetterGlitch 
          config={{
            glitchColors: ['#06b6d4', '#10b981', '#3b82f6'],
            glitchSpeed: 30,
            smooth: true
          }}
        />
      </div>
      
      <nav className="z-50 w-full px-6 md:px-10 py-6 md:py-8 flex items-center justify-between border-b border-white/5 bg-v-bg/70 backdrop-blur-2xl sticky top-0 shadow-2xl">
        <Link to="/" className="flex items-center space-x-3 md:space-x-5 group">
          <div className="w-10 h-10 md:w-14 md:h-14 rounded-xl md:rounded-2xl bg-gradient-to-br from-v-accent to-v-success flex items-center justify-center shadow-2xl shadow-v-accent/20 group-hover:scale-105 transition-all duration-500 relative shrink-0">
            <span className="text-v-bg font-bold font-display text-xl md:text-2xl">V</span>
            <div className="absolute -top-1 -right-1 w-2.5 h-2.5 md:w-3 h-3 bg-v-success rounded-full border-2 border-v-bg"></div>
          </div>
          <div className="flex flex-col min-w-0">
            <span className="text-white font-display font-bold text-lg md:text-2xl tracking-tighter group-hover:text-v-accent transition-colors truncate">Velarix Protocol</span>
            <span className="text-[8px] md:text-[10px] font-mono text-v-accent uppercase tracking-[0.2em] md:tracking-[0.4em] font-bold truncate">End-User Reasoning Guide</span>
          </div>
        </Link>

        {/* Desktop Links */}
        <div className="hidden xl:flex items-center space-x-12">
          <div className="flex items-center space-x-10">
            <a href="#" className="text-[11px] font-bold text-v-text-muted hover:text-white transition-all uppercase tracking-[0.2em] flex items-center group">
              <Network className="w-3 h-3 mr-2 opacity-30 group-hover:opacity-100 transition-opacity" /> Global Grid
            </a>
            <Link to="/dashboard" className="text-[11px] font-bold text-v-accent hover:text-v-accent/80 transition-all uppercase tracking-[0.2em]">Console Access</Link>
          </div>
          <Link to="/" className="px-8 py-3 rounded-full border border-white/10 text-[11px] font-bold hover:bg-white/5 transition-all uppercase tracking-[0.3em] shadow-inner">
            Exit Portal
          </Link>
        </div>

        {/* Mobile Menu Toggle */}
        <div className="xl:hidden flex items-center space-x-4">
           <button 
             onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
             className="p-2 text-v-text-muted hover:text-white transition-colors"
           >
             {isMobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
           </button>
        </div>

        {/* Mobile Menu Overlay */}
        <AnimatePresence>
          {isMobileMenuOpen && (
            <motion.div
              initial={{ opacity: 0, y: -20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
              className="absolute top-full left-0 w-full bg-[#0a0a0a] border-b border-white/10 p-8 flex flex-col space-y-6 xl:hidden shadow-2xl z-40"
            >
              <a href="#" onClick={() => setIsMobileMenuOpen(false)} className="text-lg font-medium text-v-text-muted hover:text-white flex items-center">
                <Network className="w-5 h-5 mr-3 text-v-accent" /> Global Grid
              </a>
              <Link to="/dashboard" onClick={() => setIsMobileMenuOpen(false)} className="text-lg font-medium text-v-accent">Console Access</Link>
              <div className="h-px w-full bg-white/5"></div>
              <Link to="/" onClick={() => setIsMobileMenuOpen(false)} className="text-lg font-medium text-white">Exit Portal</Link>
            </motion.div>
          )}
        </AnimatePresence>
      </nav>

      {/* Main Layout Container */}
      <main className="flex-1 py-10 md:py-20 px-6 md:px-16 relative z-10 w-full max-w-screen-2xl mx-auto flex flex-col lg:flex-row gap-10 lg:gap-20">
        
        {/* Mobile Sidebar Toggle */}
        <div className="lg:hidden mb-4">
           <button 
             onClick={() => setIsSidebarOpen(!isSidebarOpen)}
             className="w-full flex items-center justify-between p-4 rounded-2xl bg-v-bg-card border border-white/10 text-sm font-bold text-white shadow-lg"
           >
             <div className="flex items-center">
                <BookOpen className="w-4 h-4 mr-3 text-v-accent" />
                <span>Documentation Menu</span>
             </div>
             <ChevronDown className={`w-4 h-4 transition-transform duration-300 ${isSidebarOpen ? 'rotate-180' : ''}`} />
           </button>
        </div>

        {/* Sidebar */}
        <aside className={`${isSidebarOpen ? 'block' : 'hidden'} lg:block w-full lg:w-80 shrink-0`}>
          <div className="lg:sticky lg:top-40 space-y-10 md:space-y-16">
            <div>
              <h3 className="hidden lg:flex text-[10px] font-bold text-zinc-500 tracking-[0.4em] uppercase mb-10 items-center">
                <div className="w-10 h-px bg-v-accent mr-5"></div> Knowledge Base
              </h3>
              <nav className="space-y-2 md:space-y-3">
                {DOCS_NAV.map((tab) => (
                  <button
                    key={tab.id}
                    onClick={() => {
                      setActiveDocTab(tab.id);
                      setIsSidebarOpen(false);
                    }}
                    className={`flex items-center w-full text-left py-3.5 md:py-4 px-5 md:px-6 rounded-2xl text-xs md:text-[13px] font-bold transition-all group relative overflow-hidden ${
                      activeDocTab === tab.id 
                        ? 'bg-v-accent/10 text-v-accent border border-v-accent/20 shadow-lg shadow-v-accent/5' 
                        : 'text-v-text-muted hover:text-white hover:bg-white/5 border border-transparent'
                    }`}
                  >
                    {activeDocTab === tab.id && (
                      <motion.div layoutId="activeTab" className="absolute left-0 w-1 h-6 bg-v-accent rounded-r-full" />
                    )}
                    <tab.icon className={`w-4 h-4 mr-4 md:mr-5 transition-all duration-500 ${activeDocTab === tab.id ? 'text-v-accent scale-125' : 'opacity-20 group-hover:opacity-80 group-hover:scale-110'}`} />
                    {tab.label}
                  </button>
                ))}
              </nav>
            </div>

            <div className="hidden lg:block p-10 rounded-[2.5rem] bg-v-bg-card border border-white/5 relative overflow-hidden group shadow-[0_20px_50px_rgba(0,0,0,0.5)]">
              <div className="absolute top-0 right-0 w-40 h-40 bg-v-success/10 blur-[80px] rounded-full -mr-20 -mt-20 group-hover:bg-v-success/20 transition-all duration-700"></div>
              <h4 className="text-[10px] font-bold text-white mb-4 flex items-center tracking-[0.3em] uppercase">
                <Zap className="w-3 h-3 mr-3 text-v-success" /> Live Cluster
              </h4>
              <p className="text-[11px] text-v-text-muted leading-relaxed mb-6 font-medium">VX-Cluster-Alpha is currently active. 4.2B beliefs verified across all reasoning nodes.</p>
              <div className="flex items-center text-[10px] font-bold font-mono text-v-success uppercase tracking-[0.2em]">
                <div className="w-2 h-2 rounded-full bg-v-success mr-3 animate-pulse shadow-[0_0_10px_#10b981]"></div>
                Network Stable
              </div>
            </div>
          </div>
        </aside>

        {/* Content Area */}
        <div className="flex-1 min-w-0">
          <div className="glass-panel rounded-3xl md:rounded-[3.5rem] p-6 md:p-16 lg:p-24 min-h-[600px] md:min-h-[900px] border-white/5 shadow-[0_0_150px_rgba(0,0,0,0.4)] relative overflow-hidden">
            {/* Structural corner decorations */}
            <div className="absolute top-0 right-0 p-8 md:p-16 flex space-x-2 md:space-x-3 opacity-20">
              <div className="w-1.5 h-1.5 md:w-2 md:h-2 rounded-full bg-v-text-muted"></div>
              <div className="w-1.5 h-1.5 md:w-2 md:h-2 rounded-full bg-v-text-muted"></div>
              <div className="w-1.5 h-1.5 md:w-2 md:h-2 rounded-full bg-v-text-muted"></div>
            </div>
            
            <div className="absolute bottom-0 left-0 p-16 opacity-10 hidden md:block">
              <div className="w-48 h-px bg-v-text-muted"></div>
              <div className="w-px h-48 bg-v-text-muted mt-[-192px]"></div>
            </div>

            <AnimatePresence mode="wait">
              <motion.div
                key={activeDocTab}
                initial={{ opacity: 0, x: 20 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0, x: -20 }}
                transition={{ duration: 0.4, ease: "easeOut" }}
                className="relative z-10"
              >
                {renderContent()}
              </motion.div>
            </AnimatePresence>
            
            <footer className="mt-20 md:mt-32 pt-12 md:pt-16 border-t border-white/5 flex flex-col md:flex-row justify-between items-center gap-8 md:gap-10 relative z-10">
              <div className="text-[9px] md:text-[11px] font-bold text-v-text-muted flex items-center uppercase tracking-[0.3em] opacity-60">
                <Box className="w-3 h-3 md:w-4 md:h-4 mr-3 md:mr-4 opacity-50" />
                March 2026 • VX-PRODUCTION
              </div>
              <div className="flex flex-wrap justify-center gap-6 md:gap-10">
                <button className="text-[9px] md:text-[11px] font-bold text-v-text-muted hover:text-white uppercase tracking-widest transition-all hover:scale-105">Ecosystem Status</button>
                <button className="text-[9px] md:text-[11px] font-bold text-v-accent hover:text-white uppercase tracking-widest transition-all hover:scale-105">Developer Support</button>
              </div>
            </footer>
          </div>
        </div>
      </main>
    </div>
  );
}
