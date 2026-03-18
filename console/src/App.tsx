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
  Database, 
  AlertTriangle, 
  GitBranch, 
  Zap, 
  Clock, 
  Layers, 
  FileText, 
  Activity,
  ChevronRight,
  ShieldCheck,
  ShieldAlert,
  Key,
  BookOpen,
  LayoutDashboard,
  Settings as SettingsIcon,
  LogOut,
  User as UserIcon,
  Menu,
  X
} from 'lucide-react';
import type { Fact, JournalEntry, User, UsageStats } from './lib/types';

type Page = 'landing' | 'login' | 'signup' | 'dashboard' | 'visualizer' | 'docs' | 'settings' | 'keys';

function App() {
  const { 
    apiKey: systemApiKey,
    connect,
    authRequired: systemAuthRequired,
    sessionId, 
    setSessionId, 
    sessions,
    activeSessionInfo,
    facts, 
    history, 
    error, 
    invalidateFact, 
    getImpact, 
    getWhy 
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

  // Mock stats
  const stats: UsageStats = useMemo(() => ({
    totalFacts: Object.keys(facts).length + 1240,
    totalRequests: 8420,
    activeSessions: sessions.length || 1
  }), [facts, sessions]);

  // Route Guard
  useEffect(() => {
    const isAuthPage = ['landing', 'login', 'signup'].includes(currentPage);
    if (!user && !isAuthPage) {
      setCurrentPage('landing');
    } else if (user && isAuthPage) {
      setCurrentPage('dashboard');
    }
  }, [user, currentPage]);

  // Filtered facts for violations view
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

  if (currentPage === 'landing') {
    return <Landing 
      onGetStarted={() => setCurrentPage('signup')} 
      onLogin={() => setCurrentPage('login')} 
      onDocs={() => setCurrentPage('docs')}
    />;
  }

  if (currentPage === 'login' || currentPage === 'signup') {
    return <Auth 
      mode={currentPage as any} 
      onSuccess={handleAuthSuccess}
      onToggleMode={() => setCurrentPage(currentPage === 'login' ? 'signup' : 'login')}
    />;
  }

  if (currentPage === 'docs') {
    return <Docs onBack={() => setCurrentPage(user ? 'dashboard' : 'landing')} />;
  }

  if (currentPage === 'keys') {
    return <Keys onBack={() => setCurrentPage('dashboard')} baseUrl="http://localhost:8080" />;
  }

  return (
    <div className="flex h-screen w-screen bg-[#020617] text-slate-200 overflow-hidden font-sans selection:bg-indigo-500/30">
      
      {/* Sidebar Navigation */}
      <aside className="w-64 border-r border-white/5 bg-[#020617] flex flex-col shrink-0">
        <div className="p-6 border-b border-white/5 flex items-center gap-3">
          <div className="w-8 h-8 bg-indigo-600 rounded-lg flex items-center justify-center shadow-lg shadow-indigo-500/20">
            <Database className="w-5 h-5 text-white" />
          </div>
          <div>
            <h1 className="text-lg font-bold tracking-tight text-white leading-none">Velarix</h1>
            <span className="text-[10px] text-slate-500 font-bold uppercase tracking-widest leading-none mt-1 inline-block">SaaS Console</span>
          </div>
        </div>

        <nav className="p-4 flex-1 space-y-1">
          <NavButton 
            active={currentPage === 'dashboard'} 
            onClick={() => setCurrentPage('dashboard')}
            icon={LayoutDashboard}
            label="Dashboard"
          />
          <NavButton 
            active={currentPage === 'visualizer'} 
            onClick={() => setCurrentPage('visualizer')}
            icon={Zap}
            label="Visualizer"
          />
          <NavButton 
            active={currentPage === 'docs'} 
            onClick={() => setCurrentPage('docs')}
            icon={BookOpen}
            label="Documentation"
          />
          <div className="pt-4 pb-2 px-4">
            <span className="text-[10px] font-bold text-slate-600 uppercase tracking-[0.2em]">Management</span>
          </div>
          <NavButton 
            active={currentPage === 'keys'} 
            onClick={() => setCurrentPage('keys')}
            icon={Key}
            label="API Keys"
          />
          <NavButton 
            active={currentPage === 'settings'} 
            onClick={() => setCurrentPage('settings')}
            icon={SettingsIcon}
            label="Settings"
          />
        </nav>

        <div className="p-4 border-t border-white/5 space-y-4">
           <div className="flex items-center gap-3 px-4 py-2 bg-white/[0.02] border border-white/5 rounded-2xl">
              <div className="w-8 h-8 bg-indigo-500/10 rounded-full flex items-center justify-center border border-indigo-500/20">
                <UserIcon className="w-4 h-4 text-indigo-400" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-xs font-bold text-white truncate">{user?.name}</p>
                <p className="text-[10px] text-slate-500 truncate">{user?.email}</p>
              </div>
              <button 
                onClick={handleLogout}
                className="p-2 hover:bg-red-500/10 hover:text-red-400 rounded-lg transition-all text-slate-600"
              >
                <LogOut className="w-4 h-4" />
              </button>
           </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 flex flex-col min-w-0 relative bg-[#020617]">
        {currentPage === 'dashboard' && user && (
          <Dashboard 
            user={user} 
            stats={stats} 
            onOpenVisualizer={() => setCurrentPage('visualizer')} 
          />
        )}

        {currentPage === 'settings' && user && (
          <Settings user={user} onUpdate={setUser} />
        )}

        {currentPage === 'visualizer' && (
          <div className="flex flex-col h-full">
            {/* Visualizer Header */}
            <header className="h-16 border-b border-white/5 flex items-center px-8 gap-8 shrink-0 bg-[#020617]/80 backdrop-blur-md z-10">
              <div className="flex items-center gap-3 bg-white/[0.03] rounded-xl px-4 py-2 border border-white/5">
                <span className="text-[10px] font-bold text-slate-500 uppercase tracking-widest">Active Session</span>
                <input 
                  type="text" 
                  value={sessionId}
                  onChange={(e) => setSessionId(e.target.value)}
                  className="bg-transparent border-none text-sm font-mono text-indigo-400 focus:ring-0 p-0 w-40"
                  placeholder="session-id"
                />
              </div>
              
              <div className="h-4 w-px bg-white/5" />

              <div className="flex items-center gap-6">
                <div className="flex items-center gap-2">
                  {activeSessionInfo?.enforcement_mode === 'strict' ? <ShieldCheck className="w-4 h-4 text-indigo-400" /> : <ShieldAlert className="w-4 h-4 text-amber-400" />}
                  <span className="text-xs text-slate-400">Mode: <span className="text-slate-200 font-bold uppercase">{activeSessionInfo?.enforcement_mode || 'unknown'}</span></span>
                </div>
                <div className="flex items-center gap-2 text-slate-500">
                  <span className="text-[10px] px-1.5 py-0.5 bg-white/5 rounded border border-white/5 uppercase font-bold tracking-tighter">Live</span>
                </div>
              </div>

              <div className="ml-auto flex bg-white/[0.03] rounded-xl p-1 border border-white/5">
                {[
                  { id: 'facts', label: 'Facts', icon: FileText },
                  { id: 'violations', label: 'Violations', icon: AlertTriangle },
                  { id: 'journal', label: 'Journal', icon: Clock },
                  { id: 'graph', label: 'Graph', icon: GitBranch },
                ].map(tab => (
                  <button
                    key={tab.id}
                    onClick={() => setActiveTab(tab.id as any)}
                    className={`flex items-center gap-2 px-4 py-1.5 rounded-lg text-xs font-bold transition-all ${
                      activeTab === tab.id 
                        ? 'bg-white text-black shadow-lg shadow-white/5' 
                        : 'text-slate-500 hover:text-slate-300'
                    }`}
                  >
                    <tab.icon className={`w-3.5 h-3.5 ${activeTab === tab.id ? '' : ''}`} />
                    {tab.label}
                  </button>
                ))}
              </div>
            </header>

            {/* Visualizer Body */}
            <div className="flex-1 overflow-hidden flex relative">
              <div className="flex-1 overflow-y-auto min-w-0 bg-[#020617]">
                {activeTab === 'graph' && (
                  <div className="w-full h-full relative">
                    <Graph 
                      facts={facts} 
                      onNodeClick={setSelectedFactId} 
                      impactedNodeIds={impactedNodeIds}
                      provenanceNodeIds={provenanceNodeIds}
                    />
                  </div>
                )}

                {activeTab === 'facts' && (
                  <div className="p-8 grid grid-cols-1 xl:grid-cols-2 gap-4">
                    {Object.values(facts).length === 0 && (
                      <EmptyState icon={FileText} message="No valid facts in current context" />
                    )}
                    {Object.values(facts).map(f => (
                      <FactCard 
                        key={f.ID} 
                        fact={f} 
                        isSelected={selectedFactId === f.ID}
                        onClick={() => setSelectedFactId(f.ID)} 
                      />
                    ))}
                  </div>
                )}

                {activeTab === 'violations' && (
                  <div className="p-8 max-w-4xl mx-auto space-y-4">
                    {violations.length === 0 && (
                      <EmptyState icon={ShieldCheck} message="Clear system state. No violations recorded." />
                    )}
                    {violations.map(f => (
                      <div key={f.ID} className="bg-red-900/10 border border-red-500/20 rounded-[2rem] p-8">
                        <div className="flex items-center gap-3 mb-6">
                          <AlertTriangle className="w-6 h-6 text-red-500" />
                          <h3 className="text-xl font-bold text-white tracking-tight">Violation: {f.ID}</h3>
                        </div>
                        <div className="space-y-3">
                          {f.validation_errors?.map((err, i) => (
                            <div key={i} className="flex items-start gap-4 bg-[#020617] p-4 rounded-2xl border border-red-500/10">
                              <div className="w-1.5 h-1.5 rounded-full bg-red-500 mt-2 shrink-0" />
                              <p className="text-sm text-red-200/70 font-mono leading-relaxed">{err}</p>
                            </div>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                {activeTab === 'journal' && (
                  <div className="p-12 max-w-3xl mx-auto">
                    <div className="relative border-l border-white/5 ml-4 space-y-10 pb-12">
                      {sortedHistory.map((entry, i) => (
                        <div key={i} className="relative pl-10">
                          <div className={`absolute -left-2.5 top-0 w-5 h-5 rounded-full border-4 border-[#020617] ${
                            entry.type === 'assert' ? 'bg-indigo-500 shadow-[0_0_10px_rgba(99,102,241,0.5)]' : 
                            entry.type === 'invalidate' ? 'bg-red-500 shadow-[0_0_10px_rgba(239,68,68,0.5)]' : 'bg-slate-500'
                          }`} />
                          <div className="text-[10px] text-slate-600 font-bold uppercase tracking-widest mb-2">
                            {new Date(entry.timestamp).toLocaleTimeString()} • {entry.type}
                          </div>
                          <div className="bg-white/[0.02] border border-white/5 rounded-2xl p-5 hover:bg-white/[0.04] transition-all">
                            <div className="font-mono text-xs text-white font-bold mb-3 flex items-center justify-between">
                              {entry.fact_id || entry.fact?.ID}
                              <ChevronRight className="w-3 h-3 text-slate-700" />
                            </div>
                            {entry.fact?.payload && (
                              <div className="text-[11px] text-slate-500 bg-[#020617] p-3 rounded-xl border border-white/5 overflow-x-auto font-mono">
                                {Object.entries(entry.fact.payload).map(([k, v]) => `${k}: ${v}`).join(" • ")}
                              </div>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>

              {/* Right Sidebar: Inspector */}
              <aside className={`
                w-96 border-l border-white/5 bg-[#020617] shrink-0 transition-all duration-500 ease-out
                ${selectedFactId ? 'translate-x-0 opacity-100' : 'translate-x-full opacity-0 absolute right-0 top-0 bottom-0'}
              `}>
                {selectedFactId && facts[selectedFactId] && (
                  <Inspector 
                    fact={facts[selectedFactId]} 
                    onClose={() => setSelectedFactId(null)}
                    onInvalidate={invalidateFact}
                    onAnalyzeImpact={handleAnalyzeImpact}
                    onShowProvenance={handleShowProvenance}
                    facts={facts}
                  />
                )}
              </aside>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}

// HELPERS & SUB-COMPONENTS

function NavButton({ active, onClick, icon: Icon, label }: any) {
  return (
    <button 
      onClick={onClick}
      className={`w-full flex items-center gap-3 px-4 py-2.5 rounded-xl text-sm font-bold transition-all ${
        active 
          ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-500/20' 
          : 'text-slate-500 hover:text-slate-200 hover:bg-white/[0.02]'
      }`}
    >
      <Icon className={`w-4 h-4 ${active ? '' : 'text-slate-600'}`} />
      {label}
    </button>
  );
}

function EmptyState({ icon: Icon, message }: any) {
  return (
    <div className="col-span-full py-32 text-center opacity-20">
      <Icon className="w-16 h-16 mx-auto mb-6" />
      <p className="text-xl font-bold tracking-tight uppercase tracking-widest">{message}</p>
    </div>
  );
}

function FactCard({ fact, isSelected, onClick }: { fact: Fact, isSelected: boolean, onClick: () => void }) {
  const isValid = fact.resolved_status === 1;
  return (
    <div 
      onClick={onClick}
      className={`group p-6 rounded-[2rem] border transition-all duration-500 cursor-pointer ${
        isSelected ? 'bg-indigo-600/10 border-indigo-500 shadow-2xl' : 'bg-white/[0.02] border-white/5 hover:border-white/10'
      }`}
    >
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4 min-w-0">
          <div className={`w-2.5 h-2.5 rounded-full shrink-0 ${isValid ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]' : 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.5)]'}`} />
          <h3 className="font-bold text-white text-sm truncate tracking-tight">{fact.ID}</h3>
        </div>
        <span className={`text-[9px] font-bold px-2 py-0.5 rounded-full uppercase tracking-widest border ${fact.IsRoot ? 'border-indigo-500/30 text-indigo-400 bg-indigo-500/5' : 'border-slate-800 text-slate-500'}`}>
          {fact.IsRoot ? 'Primitive' : 'Derived'}
        </span>
      </div>
      <div className="bg-[#020617] rounded-2xl p-4 border border-white/5">
        <p className="text-[11px] text-slate-500 line-clamp-3 leading-relaxed font-mono italic">
          {typeof fact.payload === 'object' ? Object.entries(fact.payload).map(([k, v]) => `${k}: ${v}`).join(" • ") : String(fact.payload)}
        </p>
      </div>
    </div>
  );
}

function Inspector({ fact, onClose, onInvalidate, onAnalyzeImpact, onShowProvenance, facts }: any) {
  const isValid = fact.resolved_status === 1;
  return (
    <div className="flex flex-col h-full overflow-hidden bg-[#020617] border-l border-white/5 shadow-2xl">
      <div className="p-8 border-b border-white/5 flex items-center justify-between">
        <h2 className="text-xl font-bold text-white tracking-tight truncate pr-4">{fact.ID}</h2>
        <button onClick={onClose} className="p-2 hover:bg-white/5 rounded-full transition-colors text-slate-500 hover:text-white">✕</button>
      </div>
      
      <div className="flex-1 overflow-y-auto p-8 space-y-10">
        <div>
          <label className="text-[10px] font-bold text-slate-600 uppercase tracking-[0.2em] block mb-4">Status Matrix</label>
          <div className="grid grid-cols-2 gap-3">
            <div className={`p-4 rounded-2xl border transition-all ${isValid ? 'bg-emerald-500/5 border-emerald-500/20' : 'bg-red-500/5 border-red-500/20'}`}>
              <span className={`text-xs font-bold uppercase tracking-widest ${isValid ? 'text-emerald-400' : 'text-red-400'}`}>{isValid ? 'Valid' : 'Collapsed'}</span>
              <span className="block text-[9px] text-slate-600 mt-1 uppercase font-bold tracking-tighter">Current State</span>
            </div>
            <div className="p-4 rounded-2xl border border-white/5 bg-white/[0.02]">
              <span className="text-xs font-bold text-slate-300 uppercase tracking-widest">{fact.IsRoot ? 'Primitive' : 'Derived'}</span>
              <span className="block text-[9px] text-slate-600 mt-1 uppercase font-bold tracking-tighter">Fact Type</span>
            </div>
          </div>
        </div>

        <div>
          <label className="text-[10px] font-bold text-slate-600 uppercase tracking-[0.2em] block mb-4">Epistemic Payload</label>
          <div className="bg-[#020617] rounded-2xl p-5 border border-white/5 font-mono text-xs leading-relaxed text-indigo-300/80 whitespace-pre-wrap max-h-80 overflow-y-auto shadow-inner">
            {JSON.stringify(fact.payload, null, 2)}
          </div>
        </div>

        <div className="grid grid-cols-1 gap-3">
          <button 
            onClick={onShowProvenance}
            className="group w-full flex items-center justify-between gap-2 px-6 py-4 bg-indigo-600 hover:bg-indigo-500 text-white rounded-[1.5rem] text-xs font-bold transition-all shadow-xl shadow-indigo-500/20"
          >
            <div className="flex items-center gap-3">
              <GitBranch className="w-4 h-4" />
              Compute Provenance
            </div>
            <ChevronRight className="w-4 h-4 opacity-50 group-hover:translate-x-1 transition-all" />
          </button>
          <button 
            onClick={onAnalyzeImpact}
            className="group w-full flex items-center justify-between gap-2 px-6 py-4 bg-white/[0.03] hover:bg-white/[0.06] text-slate-200 border border-white/10 rounded-[1.5rem] text-xs font-bold transition-all"
          >
            <div className="flex items-center gap-3">
              <Zap className="w-4 h-4 text-amber-400" />
              Analyze Impact Radius
            </div>
            <ChevronRight className="w-4 h-4 opacity-50 group-hover:translate-x-1 transition-all" />
          </button>
        </div>

        {fact.justification_sets && fact.justification_sets.length > 0 && (
          <div>
            <label className="text-[10px] font-bold text-slate-600 uppercase tracking-[0.2em] block mb-4">Justification Logic</label>
            <div className="space-y-4">
              {fact.justification_sets.map((set: string[], i: number) => (
                <div key={i} className="p-5 bg-white/[0.01] rounded-2xl border border-white/5 hover:border-white/10 transition-colors">
                  <div className="text-[9px] font-bold text-slate-700 uppercase tracking-widest mb-3 flex items-center gap-2">
                    <ShieldCheck className="w-3 h-3" /> Logical Set {i+1}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {set.map(id => (
                      <span key={id} className={`px-2.5 py-1 rounded-lg text-[10px] font-mono font-bold transition-all border ${facts[id]?.resolved_status === 1 ? 'border-emerald-500/30 text-emerald-400 bg-emerald-500/5' : 'border-red-500/30 text-red-400 bg-red-500/5'}`}>
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
        <div className="p-8 border-t border-white/5 bg-red-500/[0.02]">
          <button 
            onClick={() => { if(confirm('Trigger causal collapse for this root?')) onInvalidate(fact.ID); }}
            className="w-full py-4 bg-red-950/20 hover:bg-red-600 text-red-400 hover:text-white border border-red-500/20 rounded-2xl text-xs font-bold transition-all flex items-center justify-center gap-2"
          >
            <AlertTriangle className="w-4 h-4" />
            Force Causal Invalidation
          </button>
        </div>
      )}
    </div>
  );
}

export default App;
