import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Link } from 'react-router-dom';
import { 
  Shield, Activity, Zap, Layers, Lock, Cpu, Code, Server, 
  CheckCircle, ChevronRight, FileText, Briefcase, Globe,
  BookOpen, Terminal, ShieldCheck, Box, Menu, X
} from 'lucide-react';
import PixelBlast from '../components/reactbits/PixelBlast';
import LetterGlitch from '../components/reactbits/LetterGlitch';
import BorderGlow from '../components/reactbits/BorderGlow';
import Lanyard from '../components/reactbits/Lanyard';


const INDUSTRIES = [
  { text: "Healthcare", color: "from-v-accent to-v-success", underline: "bg-v-success", icon: <Activity className="w-6 h-6 z-10 mx-auto mt-4 text-v-success" /> },
  { text: "Financial Services", color: "from-blue-400 to-indigo-500", underline: "bg-blue-500", icon: <Briefcase className="w-6 h-6 z-10 mx-auto mt-4 text-blue-500" /> },
  { text: "Defense & Intel", color: "from-amber-400 to-orange-500", underline: "bg-amber-500", icon: <Shield className="w-6 h-6 z-10 mx-auto mt-4 text-amber-500" /> },
  { text: "Legal & Discovery", color: "from-purple-400 to-pink-500", underline: "bg-purple-500", icon: <FileText className="w-6 h-6 z-10 mx-auto mt-4 text-purple-500" /> },
  { text: "Agentic Dev Teams", color: "from-pink-400 to-rose-500", underline: "bg-pink-500", icon: <Cpu className="w-6 h-6 z-10 mx-auto mt-4 text-pink-500" /> },
];

const INTEGRATION_STEPS = [
  { 
    title: "Initialize a Session", 
    desc: "Create an isolated reasoning session per user or workflow via the Velarix API or Dashboard. Each session is fully encrypted and org-scoped." 
  },
  { 
    title: "Swap One Import", 
    desc: "Replace your existing OpenAI or Anthropic import with Velarix's drop-in adapter. No refactoring, no new architecture — one line change." 
  },
  { 
    title: "Assert Beliefs", 
    desc: "Your agent's inputs, outputs, and derived conclusions are automatically captured and validated against the epistemic kernel in real time." 
  },
  { 
    title: "Audit Everything", 
    desc: "Every decision is causally linked. When a root fact changes, every downstream belief collapses instantly. Full SOC2-ready audit trail included." 
  }
];

const FEATURES = [
  {
    icon: <Activity className="w-8 h-8 text-v-accent mb-6" />,
    title: "Causal Invalidation Engine",
    desc: "When a foundational fact changes — like a patient revoking consent — every downstream belief that depended on it collapses automatically. No stale reasoning, ever.",
    glow: "rgba(6, 182, 212, 0.4)"
  },
  {
    icon: <Shield className="w-8 h-8 text-v-success mb-6" />,
    title: "Hardened Audit Trail",
    desc: "AES-256 encryption by default. Every state change is cryptographically attributed to an actor with a timestamp. SOC2 and HIPAA-ready out of the box.",
    glow: "rgba(16, 185, 129, 0.4)"
  },
  {
    icon: <Server className="w-8 h-8 text-purple-400 mb-6" />,
    title: "Built for Production",
    desc: "Go-powered kernel with hybrid snapshotting and journaling. O(1) session boot times. Handles thousands of concurrent agent sessions without degradation.",
    glow: "rgba(167, 139, 250, 0.4)"
  },
  {
    icon: <Layers className="w-8 h-8 text-pink-400 mb-6" />,
    title: "Schema Enforcement",
    desc: "Define strict JSON schemas per session. Malformed or hallucinated agent outputs are rejected before they can corrupt the reasoning graph.",
    glow: "rgba(244, 114, 182, 0.4)"
  },
  {
    icon: <Code className="w-8 h-8 text-indigo-400 mb-6" />,
    title: "Drop-in SDKs",
    desc: "Python and TypeScript SDKs with native LangChain, LangGraph, and LlamaIndex integrations. Async support included. Install in minutes, not days.",
    glow: "rgba(99, 102, 241, 0.4)"
  },
  {
    icon: <Cpu className="w-8 h-8 text-amber-400 mb-6" />,
    title: "Live Neural Visualizer",
    desc: "Watch your agent's reasoning graph in real time. Trace belief dependencies, replay collapse events, and export compliance reports in one click.",
    glow: "rgba(251, 191, 36, 0.4)"
  },
];

export default function LandingPage() {
  const [index, setIndex] = useState(0);
  const [activeSnippetTab, setActiveSnippetTab] = useState('python');
  const [isMenuOpen, setIsMenuOpen] = useState(false);

  useEffect(() => {
    const timer = setInterval(() => {
      setIndex((prev) => (prev + 1) % INDUSTRIES.length);
    }, 3500);
    return () => clearInterval(timer);
  }, []);

  return (
    <div className="min-h-screen bg-v-bg relative overflow-x-hidden flex flex-col font-sans text-white">
      {/* Background */}
      <div className="absolute inset-0 z-0 opacity-40 pointer-events-auto">
        <PixelBlast 
          config={{
            color: "#06b6d4", 
            liquid: true,
            liquidStrength: 0.2,
            autoPauseOffscreen: true,
            pixelSize: 4,
            speed: 0.3
          }}
        />
      </div>
      <div className="absolute top-0 right-1/4 w-[500px] h-[500px] bg-v-accent/10 rounded-full mix-blend-screen filter blur-[120px] animate-blob z-0 pointer-events-none"></div>
      <div className="absolute top-1/4 left-0 w-[400px] h-[400px] bg-v-success/10 rounded-full mix-blend-screen filter blur-[100px] animate-blob z-0 pointer-events-none" style={{ animationDelay: '2s' }}></div>
      <div className="absolute top-3/4 left-1/3 w-[600px] h-[600px] bg-purple-500/10 rounded-full mix-blend-screen filter blur-[150px] animate-blob z-0 pointer-events-none" style={{ animationDelay: '4s' }}></div>

      {/* Navbar */}
      <nav className="z-50 w-full px-6 md:px-8 py-6 flex items-center justify-between sticky top-0 bg-v-bg/80 backdrop-blur-md border-b border-white/5">
        <div className="flex items-center space-x-2">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-v-accent to-v-success flex items-center justify-center">
            <span className="text-v-bg font-bold font-display text-lg">V</span>
          </div>
          <span className="text-white font-display font-semibold text-xl tracking-wide">Velarix</span>
        </div>
        
        {/* Desktop Links */}
        <div className="hidden md:flex items-center space-x-6">
          <a href="#features" className="text-sm text-v-text-muted hover:text-white transition-colors font-medium">Features</a>
          <a href="#integration" className="text-sm text-v-text-muted hover:text-white transition-colors font-medium">How it works</a>
          <Link to="/docs" className="text-sm text-v-text-muted hover:text-white transition-colors font-medium">Docs</Link>
          <div className="h-4 w-px bg-white/10 mx-2"></div>
          <Link to="/login" className="text-sm font-medium text-white hover:text-v-accent transition-colors">Sign in</Link>
          <Link to="/signup" className="text-sm font-bold bg-white text-black px-5 py-2 rounded-full hover:bg-gray-200 transition-all shadow-[0_0_15px_rgba(255,255,255,0.2)]">Get Started</Link>
        </div>

        {/* Mobile Hamburger */}
        <button 
          onClick={() => setIsMenuOpen(!isMenuOpen)}
          className="md:hidden p-2 text-v-text-muted hover:text-white transition-colors"
        >
          {isMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
        </button>

        {/* Mobile Menu Overlay */}
        <AnimatePresence>
          {isMenuOpen && (
            <motion.div
              initial={{ opacity: 0, y: -20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -20 }}
              className="absolute top-full left-0 w-full bg-[#0a0a0a] border-b border-white/10 p-8 flex flex-col space-y-6 md:hidden shadow-2xl z-40"
            >
              <a href="#features" onClick={() => setIsMenuOpen(false)} className="text-lg font-medium text-v-text-muted hover:text-white">Features</a>
              <a href="#integration" onClick={() => setIsMenuOpen(false)} className="text-lg font-medium text-v-text-muted hover:text-white">How it works</a>
              <Link to="/docs" onClick={() => setIsMenuOpen(false)} className="text-lg font-medium text-v-text-muted hover:text-white">Docs</Link>
              <div className="h-px w-full bg-white/5"></div>
              <Link to="/login" onClick={() => setIsMenuOpen(false)} className="text-lg font-medium text-white">Sign in</Link>
              <Link to="/signup" onClick={() => setIsMenuOpen(false)} className="bg-v-accent text-v-bg text-center py-4 rounded-xl font-bold text-lg">Get Started</Link>
            </motion.div>
          )}
        </AnimatePresence>
      </nav>

      {/* Hero Section */}
      <main className="z-10 flex flex-col items-center justify-center pt-24 md:pt-32 pb-24 text-center px-6 max-w-5xl mx-auto">
        {/* Live badge */}
        <motion.div
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.8, ease: "easeOut" }}
          className="inline-flex items-center space-x-2 glass-panel px-3 py-1.5 rounded-full mb-8"
        >
          <span className="flex h-2 w-2 rounded-full bg-v-success relative">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-v-success opacity-75"></span>
          </span>
          <span className="text-xs font-mono text-v-text">Velarix v1.0 Kernel is live</span>
        </motion.div>

        {/* Static headline */}
        <motion.h1 
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.1, ease: "easeOut" }}
          className="font-display font-bold text-4xl md:text-7xl tracking-tight leading-tight text-white mb-4 drop-shadow-2xl"
        >
          Your agents make decisions.<br />Can they explain why?
        </motion.h1>

        {/* Cycling industry text */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.2, ease: "easeOut" }}
          className="relative h-[60px] md:h-[80px] mt-2 mb-2 overflow-hidden w-full max-w-[700px] mx-auto text-center"
        >
          <AnimatePresence mode="wait">
            <motion.div
              key={index}
              initial={{ y: 60, opacity: 0 }}
              animate={{ y: 0, opacity: 1 }}
              exit={{ y: -60, opacity: 0 }}
              transition={{ duration: 0.5, ease: "circOut" }}
              className="absolute inset-0 flex flex-col items-center justify-center"
            >
              <span className={`text-3xl md:text-6xl font-display font-bold text-transparent bg-clip-text bg-gradient-to-r ${INDUSTRIES[index].color}`}>
                {INDUSTRIES[index].text}
              </span>
              <motion.div 
                initial={{ width: "0%" }}
                animate={{ width: "60%" }}
                transition={{ duration: 2.5, ease: "easeInOut" }}
                className={`h-1 rounded-full mt-2 ${INDUSTRIES[index].underline}`}
              />
            </motion.div>
          </AnimatePresence>
        </motion.div>

        {/* Subheadline */}
        <motion.p 
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.3, ease: "easeOut" }}
          className="mt-8 md:mt-10 text-lg md:text-xl text-v-text-muted max-w-2xl mx-auto font-sans leading-relaxed px-2"
        >
          Velarix is the reasoning and audit layer for AI agents in regulated industries. 
          Track every belief, decision, and causal dependency — and watch them collapse the moment the truth changes.
        </motion.p>

        {/* CTAs */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.4, ease: "easeOut" }}
          className="mt-12 flex flex-col sm:flex-row items-center gap-4 w-full sm:w-auto"
        >
          <Link to="/signup" className="group w-full sm:w-auto flex items-center justify-center space-x-2 bg-white text-black px-8 py-4 rounded-full font-bold hover:bg-gray-200 transition-all font-sans shadow-lg shadow-white/10 hover:shadow-white/20">
            <span>Get Started</span>
            <svg className="w-4 h-4 group-hover:translate-x-1 transition-transform" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5l7 7m0 0l-7 7m7-7H3" />
            </svg>
          </Link>
          <Link to="/login" className="w-full sm:w-auto flex items-center justify-center px-8 py-4 rounded-full font-medium text-white glass-panel hover:bg-white/5 transition-all">
            Open Console
          </Link>
        </motion.div>
      </main>

      {/* How It Works */}
      <section id="integration" className="z-10 py-24 px-6 md:px-8 bg-black/40 border-y border-white/5 relative overflow-hidden">
        <div className="max-w-6xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl font-display font-semibold mb-4">Integrate in under 10 minutes</h2>
            <p className="text-v-text-muted max-w-2xl mx-auto">Slot Velarix into your existing agent stack without changing your architecture. One import swap and you're done.</p>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-6 relative">
            <div className="hidden md:block absolute top-[2.5rem] left-[12%] right-[12%] h-[2px] bg-gradient-to-r from-transparent via-white/10 to-transparent z-0"></div>
            {INTEGRATION_STEPS.map((step, idx) => (
              <motion.div 
                initial={{ opacity: 0, y: 30 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ duration: 0.5, delay: idx * 0.1 }}
                key={idx} 
                className="relative z-10 glass-panel p-6 rounded-2xl flex flex-col items-center text-center group hover:border-v-accent/30 transition-all duration-500"
              >
                <div className="w-12 h-12 rounded-full bg-v-bg border border-white/10 flex items-center justify-center text-v-success font-mono text-lg font-bold mb-6 shadow-[0_0_15px_rgba(16,185,129,0.1)] group-hover:scale-110 transition-transform">
                  {idx + 1}
                </div>
                <h3 className="text-lg font-semibold text-white mb-2">{step.title}</h3>
                <p className="text-sm text-v-text-muted">{step.desc}</p>
              </motion.div>
            ))}
          </div>
        </div>
      </section>

      {/* Code Snippet */}
      <section className="z-10 py-24 px-6 md:px-8 max-w-6xl mx-auto w-full">
        <div className="flex flex-col lg:flex-row items-center gap-12">
          <div className="w-full lg:w-1/2 text-left">
            <h2 className="text-3xl font-display font-semibold mb-6 leading-tight">One import.<br/>Full observability.</h2>
            <p className="text-v-text-muted leading-relaxed mb-8">
              Swap your OpenAI import for Velarix's drop-in adapter and pass a session ID. 
              From that moment, every call your agent makes is intercepted, validated, and recorded into the epistemic graph — automatically.
            </p>
            <ul className="space-y-4 mb-8">
              {[
                "Zero boilerplate — no new abstractions to learn",
                "Works with OpenAI, Anthropic, and custom endpoints",
                "Syncs directly to the encrypted Go epistemic kernel",
                "LangChain, LangGraph, and LlamaIndex native adapters"
              ].map((item, i) => (
                <li key={i} className="flex items-center space-x-3 text-sm text-zinc-300">
                  <CheckCircle className="w-5 h-5 text-v-success flex-shrink-0" />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </div>
          <div className="w-full lg:w-1/2 relative group">
            <div className="absolute -inset-1 bg-gradient-to-r from-v-accent to-v-success rounded-xl blur opacity-25 group-hover:opacity-40 transition duration-1000 group-hover:duration-200"></div>
            <div className="glass-panel rounded-xl overflow-hidden relative shadow-2xl">
              <div className="flex items-center justify-between px-4 py-3 border-b border-white/10 bg-black/50 overflow-x-auto">
                <div className="flex items-center space-x-2 shrink-0">
                  <div className="w-3 h-3 rounded-full bg-red-500/80"></div>
                  <div className="w-3 h-3 rounded-full bg-yellow-500/80"></div>
                  <div className="w-3 h-3 rounded-full bg-green-500/80"></div>
                </div>
                <div className="flex items-center space-x-2 ml-4">
                  <button onClick={() => setActiveSnippetTab('python')} className={`text-[10px] font-bold uppercase tracking-widest px-3 py-1 rounded transition-colors ${activeSnippetTab === 'python' ? 'bg-v-accent/20 text-v-accent' : 'text-zinc-500 hover:text-white'}`}>python</button>
                  <button onClick={() => setActiveSnippetTab('ts')} className={`text-[10px] font-bold uppercase tracking-widest px-3 py-1 rounded transition-colors ${activeSnippetTab === 'ts' ? 'bg-v-accent/20 text-v-accent' : 'text-zinc-500 hover:text-white'}`}>typescript</button>
                  <button onClick={() => setActiveSnippetTab('api')} className={`text-[10px] font-bold uppercase tracking-widest px-3 py-1 rounded transition-colors ${activeSnippetTab === 'api' ? 'bg-v-accent/20 text-v-accent' : 'text-zinc-500 hover:text-white'}`}>curl</button>
                </div>
              </div>
              <div className="p-6 bg-[#0d0d0d] font-mono text-xs md:text-sm leading-relaxed overflow-x-auto text-zinc-300 min-h-[220px]">
                {activeSnippetTab === 'python' && (
                  <motion.pre initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
                    <span className="text-zinc-500 line-through">from openai import OpenAI</span>{'\n'}
                    <span className="text-v-success font-bold">from velarix.adapters.openai import OpenAI</span>{'\n'}
                    {'\n'}
                    <span className="text-blue-400">client</span> = <span className="text-yellow-200">OpenAI</span>({'\n'}
                    {'  '}api_key=<span className="text-green-300">"sk-..."</span>,{'\n'}
                    {'  '}velarix_session_id=<span className="text-green-300">"user-session-001"</span>{'\n'}
                    ){'\n'}
                    {'\n'}
                    <span className="text-zinc-500"># Everything else stays the same.</span>{'\n'}
                    <span className="text-zinc-500"># Velarix handles the reasoning graph.</span>{'\n'}
                    response = <span className="text-blue-400">client</span>.chat.completions.<span className="text-yellow-200">create</span>({'\n'}
                    {'  '}model=<span className="text-green-300">"gpt-4"</span>,{'\n'}
                    {'  '}messages=[...]{'\n'}
                    ){'\n'}
                  </motion.pre>
                )}
                {activeSnippetTab === 'ts' && (
                  <motion.pre initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
                    <span className="text-zinc-500 line-through">import OpenAI from 'openai';</span>{'\n'}
                    <span className="text-v-success font-bold">import &#123; VelarixOpenAI &#125; from '@velarix/sdk';</span>{'\n'}
                    {'\n'}
                    <span className="text-blue-400">const</span> client = <span className="text-pink-400">new</span> <span className="text-yellow-200">VelarixOpenAI</span>(&#123;{'\n'}
                    {'  '}apiKey: <span className="text-green-300">'sk-...'</span>,{'\n'}
                    {'  '}velarixSessionId: <span className="text-green-300">'user-session-001'</span>{'\n'}
                    &#125;);{'\n'}
                    {'\n'}
                    <span className="text-zinc-500">// Same API. Full reasoning audit trail.</span>{'\n'}
                    <span className="text-blue-400">const</span> response = <span className="text-pink-400">await</span> client.chat.completions.<span className="text-yellow-200">create</span>(&#123;{'\n'}
                    {'  '}model: <span className="text-green-300">'gpt-4'</span>,{'\n'}
                    {'  '}messages: [...]{'\n'}
                    &#125;);{'\n'}
                  </motion.pre>
                )}
                {activeSnippetTab === 'api' && (
                  <motion.pre initial={{ opacity: 0 }} animate={{ opacity: 1 }}>
                    <span className="text-blue-400">curl</span> -X POST https://api.velarix.dev/v1/s/session-001/facts \{'\n'}
                    <span className="text-yellow-200">  -H</span> <span className="text-green-300">"Authorization: Bearer $VELARIX_KEY"</span> \{'\n'}
                    <span className="text-yellow-200">  -H</span> <span className="text-green-300">"Content-Type: application/json"</span> \{'\n'}
                    <span className="text-yellow-200">  -d</span> <span className="text-green-300">'&#123;{'\n'}
                    {'    '}"id": "consent-001",{'\n'}
                    {'    '}"payload": &#123; "status": "revoked", "patient": "P-4029" &#125;,{'\n'}
                    {'    '}"confidence": 1.0{'\n'}
                    {'  '}&#125;'</span>
                  </motion.pre>
                )}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Features */}
      <section id="features" className="z-10 py-24 px-6 md:px-8 bg-black/20 w-full relative">
        <div className="max-w-6xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl font-display font-semibold mb-4 px-2 leading-tight">Everything your agents need to be trustworthy</h2>
            <p className="text-v-text-muted max-w-2xl mx-auto px-4">Every component built for production environments where stale reasoning is a liability, not just a bug.</p>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {FEATURES.map((feature, idx) => (
              <BorderGlow key={idx} config={{ glowColor: feature.glow }} className="rounded-2xl h-full">
                <div className="glass-panel p-8 rounded-2xl relative overflow-hidden group h-full hover:bg-white/[0.03] transition-colors">
                  <div className="absolute inset-0 bg-gradient-to-br from-white/[0.03] to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
                  {feature.icon}
                  <h3 className="text-white font-medium text-lg mb-3">{feature.title}</h3>
                  <p className="text-sm text-v-text-muted leading-relaxed">{feature.desc}</p>
                </div>
              </BorderGlow>
            ))}
          </div>
        </div>
      </section>

      {/* Use Cases */}
      <section className="z-10 py-24 px-6 md:px-8 max-w-6xl mx-auto text-center w-full">
        <h2 className="text-3xl font-display font-semibold mb-4 leading-tight">Built for high-stakes workloads</h2>
        <p className="text-v-text-muted max-w-2xl mx-auto mb-16 px-4 leading-relaxed">
          Wherever an agent's wrong decision has real consequences — financial, medical, legal, or operational — 
          Velarix gives you the audit trail and causal control to catch it before it matters.
        </p>
        <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
          {INDUSTRIES.map((ind, i) => (
            <div key={i} className="glass-panel py-8 px-4 rounded-xl flex flex-col items-center hover:bg-white/5 transition-all duration-300 cursor-default group border-white/5 hover:border-v-accent/30">
              <div className="group-hover:scale-110 transition-transform duration-500 shrink-0">
                {ind.icon}
              </div>
              <span className="mt-6 font-semibold text-xs md:text-sm tracking-wide uppercase">{ind.text}</span>
            </div>
          ))}
        </div>
      </section>

      {/* Trust signals */}
      <section className="z-10 py-16 px-6 md:px-8 border-y border-white/5 bg-black/30 w-full overflow-hidden">
        <div className="max-w-4xl mx-auto text-center">
          <p className="text-[10px] font-bold text-zinc-600 uppercase tracking-[0.3em] mb-10 font-mono">Built on proven infrastructure</p>
          <div className="flex flex-wrap items-center justify-center gap-x-10 gap-y-8 opacity-40">
            {['BadgerDB', 'Go 1.23', 'OpenAI', 'Anthropic', 'LangChain', 'LangGraph', 'LlamaIndex', 'PyPI'].map((tech) => (
              <span key={tech} className="text-xs md:text-sm font-mono font-bold text-zinc-400 uppercase tracking-widest">{tech}</span>
            ))}
          </div>
        </div>
      </section>

      {/* Final CTA */}
      <section className="z-10 py-24 md:py-32 px-6 md:px-8 relative overflow-hidden border-t border-white/5 bg-black">
        <div className="absolute inset-0 z-0 opacity-20 pointer-events-none mix-blend-screen">
          <LetterGlitch 
            config={{
              glitchColors: ['#06b6d4', '#10b981', '#3b82f6'],
              glitchSpeed: 30,
              smooth: true
            }}
          />
        </div>
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full h-full max-w-[800px] bg-v-accent/5 rounded-full filter blur-[100px] pointer-events-none"></div>
        <div className="max-w-7xl mx-auto flex flex-col lg:flex-row items-center justify-between relative z-10 gap-16">
          <div className="w-full lg:w-1/2 text-center lg:text-left">
            <h2 className="text-4xl md:text-6xl font-display font-semibold mb-6 tracking-tight text-white leading-tight">
              Stop flying blind.<br/>Start tracking why.
            </h2>
            <p className="text-v-text-muted mb-10 text-lg md:text-xl leading-relaxed">
              Get your API key and have your first session running in under 10 minutes. 
              Your agents stay the same. Everything becomes explainable.
            </p>
            <div className="flex flex-col sm:flex-row items-center justify-center lg:justify-start gap-4">
              <Link to="/signup" className="w-full sm:w-auto flex items-center justify-center space-x-2 bg-white text-black px-10 py-4 rounded-full font-bold hover:bg-gray-200 transition-all shadow-[0_0_30px_rgba(255,255,255,0.15)]">
                <span>Get your API key</span>
              </Link>
              <Link to="/docs" className="w-full sm:w-auto flex items-center justify-center px-10 py-4 rounded-full font-bold text-white glass-panel hover:bg-white/10 transition-all border border-white/10">
                Read the docs
              </Link>
            </div>
          </div>
          <div className="hidden lg:block w-full lg:w-1/2 h-[500px] relative pointer-events-auto shrink-0">
            <Lanyard config={{ position: [0, -2, 20], gravity: [0, -40, 0], fov: 25 }} />
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="z-10 border-t border-white/10 py-12 px-8 flex flex-col md:flex-row justify-between items-center bg-[#050505] gap-8">
        <div className="flex items-center space-x-3">
          <div className="w-6 h-6 rounded bg-white/10 flex items-center justify-center">
            <span className="text-[10px] font-bold text-white">V</span>
          </div>
          <span className="text-white/40 text-xs font-bold font-display uppercase tracking-[0.3em]">VELARIX &copy; 2026</span>
        </div>
        <div className="flex flex-wrap justify-center items-center gap-x-8 gap-y-4">
          <a href="#" className="text-xs font-bold text-zinc-500 hover:text-white transition-colors uppercase tracking-widest">Privacy</a>
          <a href="#" className="text-xs font-bold text-zinc-500 hover:text-white transition-colors uppercase tracking-widest">Terms</a>
          <a href="#" className="text-xs font-bold text-zinc-500 hover:text-white transition-colors uppercase tracking-widest">Compliance</a>
          <Link to="/docs" className="text-xs font-bold text-v-accent hover:text-v-accent/80 transition-colors uppercase tracking-widest">Documentation</Link>
        </div>
      </footer>
    </div>
  );
}
