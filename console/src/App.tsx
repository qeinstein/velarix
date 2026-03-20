import { useState, useEffect, useMemo } from 'react';
import { useVelarix } from './useVelarix';
import { Graph } from './Graph';
import { Keys } from './Keys';
import { Docs } from './Docs';
import { Landing } from './Landing';
import { Auth } from './Auth';
import { Dashboard } from './Dashboard';
import { Settings } from './Settings';
import { 
  Database, AlertTriangle, GitBranch, Zap, Clock, FileText, 
  ChevronRight, ShieldCheck, ShieldAlert, Key, BookOpen, 
  LayoutDashboard, Settings as SettingsIcon, LogOut, User as UserIcon
} from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import type { Fact, User, UsageStats } from './lib/types';

type Page = 'landing' | 'login' | 'signup' | 'dashboard' | 'visualizer' | 'docs' | 'settings' | 'keys';

// Wrap pages in a standard transition
function PageWrapper({ children, pageKey }: { children: React.ReactNode; pageKey: string }) {
  return (
    <motion.div
      key={pageKey}
      initial={{ opacity: 0, y: 15, filter: 'blur(4px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      exit={{ opacity: 0, y: -15, filter: 'blur(4px)' }}
      transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
      className="h-full w-full flex flex-col flex-1 relative bg-[#020617]"
    >
      {children}
    </motion.div>
  );
}

function App() {
  const { 
    sessionId, setSessionId, sessions, activeSessionInfo,
    facts, history, invalidateFact, getImpact, getWhy 
  } = useVelarix();

  const [currentPage, setCurrentPage] = useState<Page>('landing');
  const [user, setUser] = useState<User | null>(() => {
    const saved = localStorage.getItem('velarix_user');
    return saved ? JSON.parse(saved) : null;
  });

  const [activeTab, setActiveTab] = useState<'facts' | 'violations' | 'journal' | 'graph'>('graph');
  const [selectedFactId, setSelectedFactId] = useState<string | null>(null);
  const [impactedNodeIds, setImpactedNodeIds] = useState<Set<string> | undefined>(undefined);
  const [provenanceNodeIds, setProvenanceNodeIds] = useState<Set<string> | undefined>(undefined);

  const stats: UsageStats = useMemo(() => ({
    totalFacts: Object.keys(facts).length,
    activeSessions: sessions.length || 1,
    journalEvents: history ? history.length : 0,
    violationCount: Object.values(facts || {}).filter(f => f.validation_errors && f.validation_errors.length > 0).length,
  }), [facts, sessions, history]);

  const recentActivity = useMemo(() => 
    [...(history || [])].sort((a, b) => b.timestamp - a.timestamp).slice(0, 5),
  [history]);

  useEffect(() => {
    const isAuthPage = ['landing', 'login', 'signup'].includes(currentPage);
    if (!user && !isAuthPage) {
      setCurrentPage('landing');
    } else if (user && isAuthPage) {
      setCurrentPage('dashboard');
    }
  }, [user, currentPage]);

  const violations = useMemo(() => 
    Object.values(facts || {}).filter(f => f.validation_errors && f.validation_errors.length > 0),
  [facts]);

  const sortedHistory = useMemo(() => 
    [...(history || [])].sort((a, b) => b.timestamp - a.timestamp),
  [history]);

  useEffect(() => {
    setImpactedNodeIds(undefined);
    setProvenanceNodeIds(undefined);
  }, [selectedFactId, sessionId]);

  const handleLogout = () => {
    localStorage.removeItem('velarix_user');
    setUser(null);
    setCurrentPage('landing');
  };

  const handleAuthSuccess = (u: User) => {
    setUser(u);
    setCurrentPage('dashboard');
  };

  const handleAnalyzeImpact = async (id: string) => {
    try {
      const report = await getImpact(id);
      setImpactedNodeIds(new Set(report.impacted_ids));
    } catch (e) {
      console.error(e);
    }
  };

  const handleShowProvenance = async (id: string) => {
    try {
      const ids = await getWhy(id);
      setProvenanceNodeIds(new Set(ids));
    } catch (e) {
      console.error(e);
    }
  };

  const renderContent = () => {
    if (currentPage === 'landing') {
      return (
        <PageWrapper pageKey="landing">
          <Landing onGetStarted={() => setCurrentPage('signup')} onLogin={() => setCurrentPage('login')} onDocs={() => setCurrentPage('docs')} />
        </PageWrapper>
      );
    }
    if (currentPage === 'login' || currentPage === 'signup') {
      return (
        <PageWrapper pageKey={currentPage}>
          <Auth mode={currentPage as any} onSuccess={handleAuthSuccess} onToggleMode={() => setCurrentPage(currentPage === 'login' ? 'signup' : 'login')} />
        </PageWrapper>
      );
    }
    if (currentPage === 'docs') {
      return (
        <PageWrapper pageKey="docs">
          <Docs onBack={() => setCurrentPage(user ? 'dashboard' : 'landing')} />
        </PageWrapper>
      );
    }
    if (currentPage === 'keys') {
      return (
        <PageWrapper pageKey="keys">
          <Keys onBack={() => setCurrentPage('dashboard')} baseUrl="http://localhost:8080" />
        </PageWrapper>
      );
    }

    return (
      <PageWrapper pageKey="app">
        <div className="flex h-screen w-screen bg-[#020617] text-slate-200 overflow-hidden font-sans selection:bg-indigo-500/30">
          <aside className="w-64 border-r border-white/5 bg-[#020617] flex flex-col shrink-0 relative z-20 shadow-2xl shadow-black/50">
            <div className="p-6 border-b border-white/5 flex items-center gap-3">
              <motion.div whileHover={{ scale: 1.05 }} className="w-8 h-8 bg-sky-500 rounded-lg flex items-center justify-center shadow-lg shadow-sky-500/20">
                <Database className="w-5 h-5 text-white" />
              </motion.div>
              <div>
                <h1 className="text-lg font-bold tracking-tight text-white leading-none">Velarix</h1>
                <span className="text-[10px] text-slate-500 font-bold uppercase tracking-widest leading-none mt-1 inline-block">Control Plane</span>
              </div>
            </div>

            <nav className="p-4 flex-1 space-y-1">
              <NavButton active={currentPage === 'dashboard'} onClick={() => setCurrentPage('dashboard')} icon={LayoutDashboard} label="Dashboard" />
              <NavButton active={currentPage === 'visualizer'} onClick={() => setCurrentPage('visualizer')} icon={Zap} label="Visualizer" />
              <NavButton active={false} onClick={() => setCurrentPage('docs')} icon={BookOpen} label="Documentation" />
              <div className="pt-4 pb-2 px-4">
                <span className="text-[10px] font-bold text-slate-600 uppercase tracking-[0.2em]">Management</span>
              </div>
              <NavButton active={false} onClick={() => setCurrentPage('keys')} icon={Key} label="API Keys" />
              <NavButton active={currentPage === 'settings'} onClick={() => setCurrentPage('settings')} icon={SettingsIcon} label="Settings" />
            </nav>

            <div className="p-4 border-t border-white/5 space-y-4">
               <motion.div whileHover={{ scale: 1.02, backgroundColor: 'rgba(255,255,255,0.04)' }} className="flex items-center gap-3 px-4 py-2 bg-white/[0.02] border border-white/5 rounded-2xl transition-colors">
                  <div className="w-8 h-8 bg-sky-500/10 rounded-full flex items-center justify-center border border-sky-500/20">
                    <UserIcon className="w-4 h-4 text-sky-400" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-xs font-bold text-white truncate">{user?.name}</p>
                    <p className="text-[10px] text-slate-500 truncate">{user?.email}</p>
                  </div>
                  <motion.button 
                    whileHover={{ scale: 1.1 }}
                    whileTap={{ scale: 0.9 }}
                    onClick={handleLogout}
                    className="p-2 hover:bg-rose-500/10 hover:text-rose-400 rounded-lg transition-all text-slate-600"
                  >
                    <LogOut className="w-4 h-4" />
                  </motion.button>
               </motion.div>
            </div>
          </aside>

          <main className="flex-1 flex flex-col min-w-0 relative bg-[#020617]">
            <AnimatePresence mode="wait">
              {currentPage === 'dashboard' && user && (
                <motion.div key="dashboard" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }} className="h-full w-full">
                  <Dashboard user={user} stats={stats} recentActivity={recentActivity} onOpenVisualizer={() => setCurrentPage('visualizer')} onOpenDocs={() => setCurrentPage('docs')} onOpenKeys={() => setCurrentPage('keys')} />
                </motion.div>
              )}

              {currentPage === 'settings' && user && (
                <motion.div key="settings" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }} className="h-full w-full">
                  <Settings user={user} onUpdate={setUser} onManageKeys={() => setCurrentPage('keys')} />
                </motion.div>
              )}

              {currentPage === 'visualizer' && (
                <motion.div key="visualizer" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }} className="flex flex-col h-full relative">
                  <header className="h-16 border-b border-white/5 flex items-center px-8 gap-8 shrink-0 bg-[#020617]/80 backdrop-blur-md z-10">
                    <div className="flex items-center gap-3 bg-white/[0.03] rounded-xl px-4 py-2 border border-white/5">
                      <span className="text-[10px] font-bold text-slate-500 uppercase tracking-widest">Active Session</span>
                      <input 
                        type="text" 
                        value={sessionId}
                        onChange={(e) => setSessionId(e.target.value)}
                        className="bg-transparent border-none text-sm font-mono text-sky-400 focus:ring-0 p-0 w-40 outline-none placeholder:text-sky-900"
                        placeholder="session-id"
                      />
                    </div>
                    
                    <div className="h-4 w-px bg-white/5" />

                    <div className="flex items-center gap-6">
                      <div className="flex items-center gap-2">
                        {activeSessionInfo?.enforcement_mode === 'strict' ? <ShieldCheck className="w-4 h-4 text-emerald-400" /> : <ShieldAlert className="w-4 h-4 text-amber-400" />}
                        <span className="text-xs text-slate-400">Mode: <span className="text-slate-200 font-bold uppercase">{activeSessionInfo?.enforcement_mode || 'unknown'}</span></span>
                      </div>
                      <div className="flex items-center gap-2 text-slate-500">
                        <span className="text-[10px] px-1.5 py-0.5 bg-sky-500/10 text-sky-400 rounded border border-sky-500/20 uppercase font-bold tracking-tighter shadow-[0_0_10px_rgba(14,165,233,0.2)]">Live</span>
                      </div>
                    </div>

                    <div className="ml-auto flex bg-white/[0.03] rounded-xl p-1 border border-white/5 relative">
                      {[
                        { id: 'facts', label: 'Facts', icon: FileText },
                        { id: 'violations', label: 'Violations', icon: AlertTriangle },
                        { id: 'journal', label: 'Journal', icon: Clock },
                        { id: 'graph', label: 'Graph', icon: GitBranch },
                      ].map(tab => (
                        <button
                          key={tab.id}
                          onClick={() => setActiveTab(tab.id as any)}
                          className={`relative flex items-center gap-2 px-4 py-1.5 rounded-lg text-xs font-bold transition-all z-10 ${
                            activeTab === tab.id ? 'text-white' : 'text-slate-500 hover:text-slate-300'
                          }`}
                        >
                          {activeTab === tab.id && (
                            <motion.div layoutId="viz-tab" className="absolute inset-0 bg-white/10 border border-white/10 rounded-lg -z-10 shadow-lg" transition={{ type: 'spring', bounce: 0.2, duration: 0.5 }} />
                          )}
                          <tab.icon className="w-3.5 h-3.5" />
                          {tab.label}
                        </button>
                      ))}
                    </div>
                  </header>

                  <div className="flex-1 overflow-hidden flex relative">
                    <div className="flex-1 overflow-y-auto min-w-0 bg-gradient-to-b from-[#020617] to-[#04091a]">
                      <AnimatePresence mode="wait">
                        {activeTab === 'graph' && (
                          <motion.div key="graph" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="w-full h-full relative">
                            {Object.keys(facts).length > 0 ? (
                              <Graph facts={facts} onNodeClick={setSelectedFactId} impactedNodeIds={impactedNodeIds} provenanceNodeIds={provenanceNodeIds} />
                            ) : (
                              <SkeletonLoader type="graph" />
                            )}
                          </motion.div>
                        )}

                        {activeTab === 'facts' && (
                          <motion.div key="facts" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="p-8 grid grid-cols-1 xl:grid-cols-2 gap-6">
                            {Object.values(facts).length === 0 && <EmptyState icon={FileText} message="No valid facts in current context" />}
                            <AnimatePresence>
                              {Object.values(facts).map(f => (
                                <FactCard key={f.ID} fact={f} isSelected={selectedFactId === f.ID} onClick={() => setSelectedFactId(f.ID)} />
                              ))}
                            </AnimatePresence>
                          </motion.div>
                        )}

                        {activeTab === 'violations' && (
                          <motion.div key="violations" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="p-8 max-w-4xl mx-auto space-y-6">
                            {violations.length === 0 && <EmptyState icon={ShieldCheck} message="Clear system state. No violations recorded." />}
                            <AnimatePresence>
                              {violations.map(f => (
                                <motion.div key={f.ID} initial={{ opacity: 0, scale: 0.95 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.95 }} className="bg-rose-900/10 border border-rose-500/20 rounded-[2rem] p-8 shadow-[0_10px_40px_rgba(251,113,133,0.05)]">
                                  <div className="flex items-center gap-3 mb-6">
                                    <AlertTriangle className="w-6 h-6 text-rose-500" />
                                    <h3 className="text-xl font-bold text-white tracking-tight">Violation: {f.ID}</h3>
                                  </div>
                                  <div className="space-y-3">
                                    {f.validation_errors?.map((err, i) => (
                                      <div key={i} className="flex items-start gap-4 bg-[#020617]/50 backdrop-blur-md p-4 rounded-2xl border border-rose-500/10">
                                        <div className="w-1.5 h-1.5 rounded-full bg-rose-500 mt-2 shrink-0 shadow-[0_0_8px_rgba(251,113,133,0.8)]" />
                                        <p className="text-sm text-rose-200/80 font-mono leading-relaxed">{err}</p>
                                      </div>
                                    ))}
                                  </div>
                                </motion.div>
                              ))}
                            </AnimatePresence>
                          </motion.div>
                        )}

                        {activeTab === 'journal' && (
                          <motion.div key="journal" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="p-12 max-w-3xl mx-auto">
                            <div className="relative border-l border-white/10 ml-4 space-y-12 pb-12">
                              {sortedHistory.length === 0 && <EmptyState icon={Clock} message="Journal is empty." />}
                              <AnimatePresence>
                                {sortedHistory.map((entry, i) => (
                                  <motion.div key={i} initial={{ opacity: 0, x: -10 }} animate={{ opacity: 1, x: 0 }} transition={{ delay: i * 0.05 }} className="relative pl-10 group">
                                    <div className={`absolute -left-2.5 top-0 w-5 h-5 rounded-full border-4 border-[#020617] transition-all group-hover:scale-125 ${
                                      entry.type === 'assert' ? 'bg-sky-500 shadow-[0_0_12px_rgba(14,165,233,0.6)]' : 
                                      entry.type === 'invalidate' ? 'bg-rose-500 shadow-[0_0_12px_rgba(251,113,133,0.6)]' : 'bg-slate-500'
                                    }`} />
                                    <div className="text-[10px] text-slate-500 font-bold uppercase tracking-widest mb-3 flex items-center gap-2">
                                      <span className="bg-white/5 px-2 py-1 rounded text-white">{new Date(entry.timestamp).toLocaleTimeString()}</span>
                                      <span className={entry.type === 'invalidate' ? 'text-rose-400' : 'text-sky-400'}>{entry.type.replace('_', ' ')}</span>
                                    </div>
                                    <div className="bg-white/[0.02] border border-white/5 rounded-2xl p-6 hover:bg-white/[0.04] hover:border-white/10 transition-all shadow-lg hover:shadow-xl hover:-translate-y-1 cursor-pointer" onClick={() => setSelectedFactId(entry.fact_id || entry.fact?.ID || null)}>
                                      <div className="font-mono text-sm text-white font-bold mb-4 flex items-center justify-between">
                                        {entry.fact_id || entry.fact?.ID}
                                        <ChevronRight className="w-4 h-4 text-slate-600 group-hover:text-sky-400 transition-colors" />
                                      </div>
                                      {entry.fact?.payload && (
                                        <div className="text-xs text-sky-200/60 bg-[#020617]/50 p-4 rounded-xl border border-white/5 overflow-x-auto font-mono">
                                          {Object.entries(entry.fact.payload).map(([k, v]) => `${k}: ${v}`).join(" • ")}
                                        </div>
                                      )}
                                    </div>
                                  </motion.div>
                                ))}
                              </AnimatePresence>
                            </div>
                          </motion.div>
                        )}
                      </AnimatePresence>
                    </div>

                    <AnimatePresence>
                      {selectedFactId && facts[selectedFactId] && (
                        <motion.aside 
                          initial={{ x: '100%', opacity: 0 }}
                          animate={{ x: 0, opacity: 1 }}
                          exit={{ x: '100%', opacity: 0 }}
                          transition={{ type: 'spring', damping: 25, stiffness: 200 }}
                          className="w-96 border-l border-white/10 bg-[#040816]/95 backdrop-blur-3xl shrink-0 absolute right-0 top-0 bottom-0 z-30 shadow-[-20px_0_60px_rgba(0,0,0,0.5)]"
                        >
                          <Inspector 
                            fact={facts[selectedFactId]} 
                            onClose={() => setSelectedFactId(null)}
                            onInvalidate={invalidateFact}
                            onAnalyzeImpact={handleAnalyzeImpact}
                            onShowProvenance={handleShowProvenance}
                            facts={facts}
                          />
                        </motion.aside>
                      )}
                    </AnimatePresence>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </main>
        </div>
      </PageWrapper>
    );
  };

  return <AnimatePresence mode="wait">{renderContent()}</AnimatePresence>;
}

// HELPERS & SUB-COMPONENTS

function NavButton({ active, onClick, icon: Icon, label }: any) {
  return (
    <button 
      onClick={onClick}
      className={`relative w-full flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-bold transition-all group overflow-hidden ${
        active 
          ? 'text-white' 
          : 'text-slate-500 hover:text-slate-200 hover:bg-white/[0.02]'
      }`}
    >
      {active && (
        <motion.div layoutId="nav-active" className="absolute inset-0 bg-sky-500/10 border border-sky-500/20 rounded-xl" transition={{ type: 'spring', bounce: 0.2, duration: 0.5 }} />
      )}
      <Icon className={`w-4 h-4 z-10 ${active ? 'text-sky-400 drop-shadow-[0_0_8px_rgba(56,189,248,0.8)]' : 'text-slate-600 group-hover:text-slate-400'}`} />
      <span className="z-10">{label}</span>
    </button>
  );
}

function EmptyState({ icon: Icon, message }: any) {
  return (
    <div className="col-span-full flex flex-col items-center justify-center py-32 text-center opacity-40">
      <Icon className="w-16 h-16 mb-6 text-slate-500" />
      <p className="text-lg font-bold uppercase tracking-widest text-slate-400">{message}</p>
    </div>
  );
}

function FactCard({ fact, isSelected, onClick }: { fact: Fact, isSelected: boolean, onClick: () => void }) {
  const isValid = fact.resolved_status === 1;
  return (
    <motion.div 
      initial={{ opacity: 0, scale: 0.95 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.95 }}
      whileHover={{ y: -4, scale: 1.01 }}
      whileTap={{ scale: 0.98 }}
      onClick={onClick}
      className={`group p-6 rounded-[2rem] border transition-all duration-300 cursor-pointer ${
        isSelected ? 'bg-sky-500/10 border-sky-500 shadow-[0_10px_40px_rgba(56,189,248,0.15)] ring-1 ring-sky-500/30' : 'bg-white/[0.02] border-white/5 hover:border-white/15 hover:shadow-xl hover:bg-white/[0.04]'
      }`}
    >
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4 min-w-0">
          <div className={`w-2.5 h-2.5 rounded-full shrink-0 ${isValid ? 'bg-emerald-400 shadow-[0_0_12px_rgba(52,211,153,0.8)]' : 'bg-rose-500 shadow-[0_0_12px_rgba(251,113,133,0.8)]'}`} />
          <h3 className="font-bold text-white text-base truncate tracking-tight">{fact.ID}</h3>
        </div>
        <span className={`text-[9px] font-bold px-3 py-1 rounded-full uppercase tracking-widest border ${fact.IsRoot ? 'border-sky-500/30 text-sky-400 bg-sky-500/10' : 'border-white/10 text-slate-400 bg-white/5'}`}>
          {fact.IsRoot ? 'Primitive' : 'Derived'}
        </span>
      </div>
      <div className="bg-[#020617]/50 rounded-[1.5rem] p-5 border border-white/5 shadow-inner">
        <p className="text-[12px] text-slate-400 line-clamp-3 leading-relaxed font-mono">
          {typeof fact.payload === 'object' ? Object.entries(fact.payload).map(([k, v]) => `${k}: ${v}`).join(" • ") : String(fact.payload)}
        </p>
      </div>
    </motion.div>
  );
}

function Inspector({ fact, onClose, onInvalidate, onAnalyzeImpact, onShowProvenance, facts }: any) {
  const isValid = fact.resolved_status === 1;
  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="p-8 border-b border-white/10 flex items-center justify-between bg-white/[0.01]">
        <h2 className="text-xl font-bold text-white tracking-tight truncate pr-4">{fact.ID}</h2>
        <motion.button whileHover={{ scale: 1.1, rotate: 90 }} whileTap={{ scale: 0.9 }} onClick={onClose} className="p-2 bg-white/5 hover:bg-white/10 rounded-full transition-colors text-slate-400 hover:text-white">✕</motion.button>
      </div>
      
      <div className="flex-1 overflow-y-auto p-8 space-y-10 custom-scrollbar">
        <div>
          <label className="text-[10px] font-bold text-slate-500 uppercase tracking-[0.2em] block mb-4">Status Matrix</label>
          <div className="grid grid-cols-2 gap-3">
            <div className={`p-4 rounded-2xl border transition-all ${isValid ? 'bg-emerald-500/10 border-emerald-500/20 shadow-[0_4px_20px_rgba(52,211,153,0.1)]' : 'bg-rose-500/10 border-rose-500/20 shadow-[0_4px_20px_rgba(251,113,133,0.1)]'}`}>
              <span className={`text-xs font-bold uppercase tracking-widest flex items-center gap-2 ${isValid ? 'text-emerald-400' : 'text-rose-400'}`}>
                <div className={`w-2 h-2 rounded-full ${isValid ? 'bg-emerald-400' : 'bg-rose-500'}`} />
                {isValid ? 'Valid' : 'Collapsed'}
              </span>
              <span className="block text-[9px] text-slate-500 mt-2 uppercase font-bold tracking-tighter">Current State</span>
            </div>
            <div className="p-4 rounded-2xl border border-white/10 bg-white/[0.03]">
              <span className="text-xs font-bold text-sky-400 uppercase tracking-widest">{fact.IsRoot ? 'Primitive' : 'Derived'}</span>
              <span className="block text-[9px] text-slate-500 mt-2 uppercase font-bold tracking-tighter">Fact Type</span>
            </div>
          </div>
        </div>

        <div>
          <label className="text-[10px] font-bold text-slate-500 uppercase tracking-[0.2em] block mb-4">Epistemic Payload</label>
          <div className="bg-[#020617] rounded-2xl p-5 border border-white/10 font-mono text-xs leading-relaxed text-sky-200/80 whitespace-pre-wrap max-h-80 overflow-y-auto shadow-inner">
            {JSON.stringify(fact.payload, null, 2)}
          </div>
        </div>

        <div className="grid grid-cols-1 gap-3">
          <motion.button 
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
            onClick={onShowProvenance}
            className="group w-full flex items-center justify-between gap-2 px-6 py-4 bg-sky-500 hover:bg-sky-400 text-white rounded-[1.5rem] text-xs font-bold transition-colors shadow-[0_10px_30px_rgba(14,165,233,0.2)]"
          >
            <div className="flex items-center gap-3">
              <GitBranch className="w-4 h-4" />
              Compute Provenance
            </div>
            <ChevronRight className="w-4 h-4 opacity-50 group-hover:translate-x-1 transition-transform" />
          </motion.button>
          <motion.button 
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
            onClick={onAnalyzeImpact}
            className="group w-full flex items-center justify-between gap-2 px-6 py-4 bg-white/[0.04] hover:bg-white/[0.08] text-slate-200 border border-white/10 rounded-[1.5rem] text-xs font-bold transition-colors"
          >
            <div className="flex items-center gap-3">
              <Zap className="w-4 h-4 text-amber-400" />
              Analyze Impact Radius
            </div>
            <ChevronRight className="w-4 h-4 opacity-50 group-hover:translate-x-1 transition-transform" />
          </motion.button>
        </div>

        {fact.justification_sets && fact.justification_sets.length > 0 && (
          <div>
            <label className="text-[10px] font-bold text-slate-500 uppercase tracking-[0.2em] block mb-4">Justification Logic</label>
            <div className="space-y-4">
              {fact.justification_sets.map((set: string[], i: number) => (
                <div key={i} className="p-5 bg-white/[0.02] rounded-2xl border border-white/10 hover:border-white/20 transition-colors">
                  <div className="text-[9px] font-bold text-slate-500 uppercase tracking-widest mb-4 flex items-center gap-2">
                    <ShieldCheck className="w-3 h-3 text-sky-400" /> Logical Set {i+1}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {set.map((id: string) => (
                      <span key={id} className={`px-3 py-1.5 rounded-lg text-[10px] font-mono font-bold transition-all border ${facts[id]?.resolved_status === 1 ? 'border-emerald-500/30 text-emerald-300 bg-emerald-500/10' : 'border-rose-500/30 text-rose-300 bg-rose-500/10'}`}>
                        {id}
                      </span>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {fact.IsRoot && isValid && (
        <div className="p-8 border-t border-white/10 bg-rose-500/[0.03]">
          <motion.button 
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
            onClick={() => { if(confirm('Trigger causal collapse for this root?')) onInvalidate(fact.ID); }}
            className="w-full py-4 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/20 rounded-2xl text-xs font-bold transition-colors flex items-center justify-center gap-2 shadow-[0_10px_30px_rgba(251,113,133,0.1)]"
          >
            <AlertTriangle className="w-4 h-4" />
            Force Causal Invalidation
          </motion.button>
        </div>
      )}
    </div>
  );
}

function SkeletonLoader({ type }: { type: 'graph' | 'list' | 'cards' }) {
  if (type === 'graph') {
    return (
      <div className="absolute inset-0 flex items-center justify-center p-8 overflow-hidden pointer-events-none">
        <div className="relative w-full h-full max-w-4xl max-h-[600px]">
          <motion.div className="absolute top-[20%] left-[30%] w-56 h-28 bg-white/[0.03] border border-white/[0.06] rounded-[1.6rem] shadow-2xl backdrop-blur-sm" animate={{ opacity: [0.3, 0.7, 0.3], y: [0, -5, 0] }} transition={{ duration: 3, repeat: Infinity, ease: "easeInOut" }} />
          <motion.div className="absolute top-[50%] left-[20%] w-56 h-28 bg-white/[0.03] border border-white/[0.06] rounded-[1.6rem] shadow-2xl backdrop-blur-sm" animate={{ opacity: [0.3, 0.7, 0.3], y: [0, -5, 0] }} transition={{ duration: 3, delay: 0.5, repeat: Infinity, ease: "easeInOut" }} />
          <motion.div className="absolute top-[60%] left-[60%] w-56 h-28 bg-white/[0.03] border border-white/[0.06] rounded-[1.6rem] shadow-2xl backdrop-blur-sm" animate={{ opacity: [0.3, 0.7, 0.3], y: [0, -5, 0] }} transition={{ duration: 3, delay: 1, repeat: Infinity, ease: "easeInOut" }} />
          <svg className="absolute inset-0 w-full h-full" style={{ zIndex: -1 }}>
            <motion.path d="M 350 200 C 350 300, 250 200, 250 300" stroke="rgba(255,255,255,0.05)" strokeWidth="2" fill="none" animate={{ strokeDashoffset: [0, 100] }} strokeDasharray="10 10" transition={{ duration: 2, repeat: Infinity, ease: "linear" }} />
            <motion.path d="M 450 200 C 450 350, 600 250, 600 350" stroke="rgba(255,255,255,0.05)" strokeWidth="2" fill="none" animate={{ strokeDashoffset: [0, 100] }} strokeDasharray="10 10" transition={{ duration: 2, repeat: Infinity, ease: "linear" }} />
          </svg>
        </div>
      </div>
    );
  }
  return null;
}

export default App;
