import { useEffect, useState } from 'react';
import { KeyRound, ShieldAlert, Cpu, Loader2, Trash2, Copy, CheckCircle, X } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

export default function Settings() {
  const [keys, setKeys] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  // Modals & UI States
  const [isGenerateModalOpen, setGenerateModalOpen] = useState(false);
  const [newKeyLabel, setNewKeyLabel] = useState('New API Key');
  const [revokeKeyTarget, setRevokeKeyTarget] = useState<string | null>(null);
  const [generateError, setGenerateError] = useState<string | null>(null);
  const [revokeError, setRevokeError] = useState<string | null>(null);

  const [toggles, setToggles] = useState({
    isolation: true,
    audit: true
  });

  const fetchKeys = async () => {
    try {
      const token = localStorage.getItem('velarix_token');
      const res = await fetch('http://localhost:8080/v1/keys', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to fetch keys');
      const data = await res.json();
      setKeys(data || []);
    } catch (err: any) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchKeys();
  }, []);

  const handleGenerateKey = async (e: React.FormEvent) => {
    e.preventDefault();
    setGenerating(true);
    setGenerateError(null);
    try {
      const token = localStorage.getItem('velarix_token');
      const res = await fetch('http://localhost:8080/v1/keys/generate', {
        method: 'POST',
        headers: { 
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ label: newKeyLabel })
      });
      if (!res.ok) throw new Error('Failed to generate key');
      await fetchKeys();
      setGenerateModalOpen(false);
      setNewKeyLabel('New API Key');
    } catch (err: any) {
      setGenerateError(err.message);
    } finally {
      setGenerating(false);
    }
  };

  const executeRevoke = async () => {
    if (!revokeKeyTarget) return;
    setRevokeError(null);
    try {
      const token = localStorage.getItem('velarix_token');
      const res = await fetch(`http://localhost:8080/v1/keys/${revokeKeyTarget}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to revoke key');
      await fetchKeys();
      setRevokeKeyTarget(null);
    } catch (err: any) {
      setRevokeError(err.message);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(text);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  // Animation variants
  const containerVars = {
    hidden: { opacity: 0 },
    show: {
      opacity: 1,
      transition: { staggerChildren: 0.1 }
    }
  };

  const itemVars = {
    hidden: { opacity: 0, y: 30 },
    show: { opacity: 1, y: 0, transition: { type: 'spring', stiffness: 300, damping: 24 } }
  };

  return (
    <motion.div 
      variants={containerVars} 
      initial="hidden" 
      animate="show" 
      className="max-w-4xl mx-auto space-y-8 pb-20 relative"
    >
      <motion.div variants={itemVars}>
        <h1 className="text-2xl font-display font-semibold text-white px-1">Platform Settings</h1>
        <p className="text-sm text-v-text-muted mt-1 px-1">Manage API keys, encryption, and organization variables.</p>
      </motion.div>

      {/* API Keys Panel */}
      <motion.section variants={itemVars} className="glass-panel rounded-xl overflow-hidden shadow-[0_0_30px_rgba(0,0,0,0.5)]">
        <div className="border-b border-v-border p-6 flex flex-col md:flex-row md:items-center justify-between gap-4">
           <div>
             <h2 className="text-lg font-medium text-white flex items-center">
               <KeyRound className="w-5 h-5 mr-3 text-v-accent" />
               API Keys
             </h2>
             <p className="text-sm text-v-text-muted mt-1">Keys used by your agents to authenticate with the Velarix Kernel.</p>
           </div>
           <button 
             onClick={() => setGenerateModalOpen(true)}
             disabled={generating}
             className="w-full md:w-auto bg-white text-black px-4 py-2.5 rounded-md font-bold text-sm hover:bg-gray-200 transition-colors shrink-0 flex items-center justify-center space-x-2 disabled:opacity-50 shadow-lg"
           >
             {generating ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
             <span>Generate New Key</span>
           </button>
        </div>
        <div className="p-0 sm:p-6">
           <div className="border-y sm:border border-v-border sm:rounded-lg overflow-x-auto bg-black/20">
             <table className="w-full text-left text-sm min-w-[600px] sm:min-w-0">
               <thead className="bg-[#050505] border-b border-v-border">
                 <tr>
                   <th className="px-4 py-3 font-medium text-v-text-muted uppercase text-[10px] tracking-widest">Label</th>
                   <th className="px-4 py-3 font-medium text-v-text-muted uppercase text-[10px] tracking-widest">Key Prefix</th>
                   <th className="px-4 py-3 font-medium text-v-text-muted uppercase text-[10px] tracking-widest hidden md:table-cell">Created</th>
                   <th className="px-4 py-3 font-medium text-v-text-muted uppercase text-[10px] tracking-widest">Status</th>
                   <th className="px-4 py-3"></th>
                 </tr>
               </thead>
               <tbody className="divide-y divide-white/5">
                  {loading ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-8 text-center text-v-text-muted">
                        <Loader2 className="w-6 h-6 animate-spin mx-auto text-v-accent" />
                      </td>
                    </tr>
                  ) : keys.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-8 text-center text-zinc-500 font-mono text-sm">No API keys found.</td>
                    </tr>
                  ) : keys.map((k: any) => (
                    <motion.tr 
                       key={k.key} 
                       initial={{ opacity: 0 }} 
                       animate={{ opacity: 1 }}
                       className={`hover:bg-white/5 transition-colors group ${k.is_revoked ? 'opacity-50' : ''}`}
                    >
                      <td className="px-4 py-4 text-white font-medium">{k.label || 'Unnamed Key'}</td>
                      <td className="px-4 py-4 font-mono text-v-text-muted text-xs">
                         <div className="flex items-center space-x-3">
                            <span className="truncate max-w-[100px] sm:max-w-none">{k.key.substring(0, 12)}••••</span>
                            <button 
                              onClick={() => copyToClipboard(k.key)}
                              className="text-zinc-500 hover:text-white transition-colors p-1 opacity-100 sm:opacity-0 group-hover:opacity-100"
                            >
                              {copiedKey === k.key ? <CheckCircle className="w-4 h-4 text-v-success" /> : <Copy className="w-4 h-4" />}
                            </button>
                         </div>
                      </td>
                      <td className="px-4 py-4 text-v-text-muted text-xs hidden md:table-cell">{new Date(k.created_at).toLocaleDateString()}</td>
                      <td className="px-4 py-4">
                        <span className={`px-2 py-0.5 rounded-full text-[10px] uppercase font-bold tracking-wide border ${k.is_revoked ? 'bg-red-500/10 text-red-400 border-red-500/30' : 'bg-v-success/10 text-v-success border-v-success/30'}`}>
                          {k.is_revoked ? 'Revoked' : 'Active'}
                        </span>
                      </td>
                      <td className="px-4 py-4 text-right">
                         {!k.is_revoked && (
                           <button onClick={() => setRevokeKeyTarget(k.key)} className="text-zinc-600 hover:text-red-400 p-1 transition-colors">
                            <Trash2 className="w-4 h-4" />
                           </button>
                         )}
                      </td>
                    </motion.tr>
                  ))}
               </tbody>
             </table>
           </div>
        </div>
      </motion.section>

      {/* Encryption & Security */}
      <motion.section variants={itemVars} className="glass-panel rounded-xl overflow-hidden p-6 border-l-2 border-l-v-success shadow-[0_0_30px_rgba(16,185,129,0.05)] relative">
         <div className="absolute top-0 left-0 w-64 h-64 bg-v-success/10 blur-[100px] pointer-events-none" />
         <div className="flex flex-col sm:flex-row items-start relative z-10 gap-4">
           <ShieldAlert className="w-6 h-6 text-v-success mt-1 shrink-0" />
           <div className="flex-1">
             <h2 className="text-lg font-medium text-white">Encryption at Rest (AES-256)</h2>
             <p className="text-sm text-zinc-400 mt-1 leading-relaxed max-w-2xl">
               Your organization is actively utilizing AES-256 encryption for the BadgerDB physical storage layer. Ensure you have backed up your primary encryption phrase.
             </p>
             <div className="mt-6 flex flex-wrap gap-4">
                <button className="px-4 py-2 bg-[#0a0a0a] border border-white/10 rounded-md text-sm text-white hover:border-v-accent transition-colors font-medium">
                  Rotate Master Key
                </button>
                <button className="px-4 py-2 border border-transparent rounded-md text-sm text-zinc-500 hover:text-white transition-colors font-medium">
                  Download Phrase
                </button>
             </div>
           </div>
         </div>
      </motion.section>

      {/* Instance Config */}
      <motion.section variants={itemVars} className="glass-panel rounded-xl overflow-hidden shadow-[0_0_30px_rgba(0,0,0,0.5)] relative">
        <div className="absolute bottom-0 right-0 w-64 h-64 bg-purple-500/5 blur-[100px] pointer-events-none" />
        <div className="border-b border-v-border p-6 flex items-center relative z-10">
            <Cpu className="w-5 h-5 mr-3 text-purple-400" />
            <h2 className="text-lg font-medium text-white">Kernel Configuration</h2>
        </div>
        <div className="p-6 space-y-6 relative z-10">
           
           <div className="flex items-center justify-between gap-4">
              <div>
                <h3 className="text-white font-medium text-sm">Strict Tenant Isolation</h3>
                <p className="text-xs text-zinc-500 mt-1">Enforce hard boundaries on session queries across org identities.</p>
              </div>
              <button 
                onClick={() => setToggles(p => ({ ...p, isolation: !p.isolation }))}
                className={`w-11 h-6 rounded-full flex items-center p-1 transition-colors shrink-0 ${toggles.isolation ? 'bg-v-success' : 'bg-white/10'}`}
              >
                 <motion.div layout className="w-4 h-4 rounded-full bg-white shadow-sm" animate={{ x: toggles.isolation ? 20 : 0 }} />
              </button>
           </div>
           
           <div className="flex items-center justify-between gap-4">
              <div>
                <h3 className="text-white font-medium text-sm">Deep Audit Logging</h3>
                <p className="text-xs text-zinc-500 mt-1">Retain full payload differences in the persistence journal.</p>
              </div>
              <button 
                onClick={() => setToggles(p => ({ ...p, audit: !p.audit }))}
                className={`w-11 h-6 rounded-full flex items-center p-1 transition-colors shrink-0 ${toggles.audit ? 'bg-v-success' : 'bg-white/10'}`}
              >
                 <motion.div layout className="w-4 h-4 rounded-full bg-white shadow-sm" animate={{ x: toggles.audit ? 20 : 0 }} />
              </button>
           </div>

           <div className="flex items-center justify-between opacity-50 gap-4">
              <div>
                <h3 className="text-white font-medium text-sm">Vector Synchronization</h3>
                <p className="text-xs text-zinc-500 mt-1">Automatic push of valid facts to external vector stores.</p>
              </div>
              <div className="flex items-center space-x-4 shrink-0">
                 <span className="text-[10px] text-v-accent font-bold tracking-widest uppercase">Coming Q3</span>
                 <div className="w-11 h-6 bg-white/5 border border-white/5 rounded-full flex items-center p-1 cursor-not-allowed">
                    <div className="w-4 h-4 rounded-full bg-zinc-600"></div>
                 </div>
              </div>
           </div>
        </div>
      </motion.section>

      {/* Frame Motion Modals */}
      <AnimatePresence>
        {isGenerateModalOpen && (
           <div className="fixed inset-0 z-[100] flex items-center justify-center p-4">
             <motion.div 
               initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
               className="absolute inset-0 bg-black/60 backdrop-blur-sm"
               onClick={() => setGenerateModalOpen(false)}
             />
             <motion.div 
               initial={{ scale: 0.95, opacity: 0, y: 20 }}
               animate={{ scale: 1, opacity: 1, y: 0 }}
               exit={{ scale: 0.95, opacity: 0, y: 20 }}
               className="relative bg-[#0a0a0a] border border-white/10 p-6 rounded-xl w-full max-w-sm shadow-2xl"
             >
                <div className="flex justify-between items-center mb-4">
                  <h3 className="text-lg font-medium text-white">Generate API Key</h3>
                  <button onClick={() => setGenerateModalOpen(false)} className="text-zinc-500 hover:text-white"><X className="w-4 h-4" /></button>
                </div>
                <form onSubmit={handleGenerateKey}>
                  <p className="text-sm text-zinc-400 mb-4 font-sans">Create a new key to allow external agents to access the Epistemic Engine.</p>
                  {generateError && <div className="p-3 mb-4 bg-red-500/20 border border-red-500/50 rounded-lg text-sm text-red-200">{generateError}</div>}
                  <label className="block text-xs font-medium text-zinc-500 mb-1 uppercase tracking-wider">Key Label</label>
                  <input 
                    type="text" 
                    value={newKeyLabel} 
                    onChange={e => setNewKeyLabel(e.target.value)}
                    className="w-full bg-black border border-white/10 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-v-accent focus:ring-1 focus:ring-v-accent transition-all font-sans"
                    autoFocus
                    required
                  />
                  <div className="mt-8 flex justify-end space-x-3">
                     <button type="button" onClick={() => setGenerateModalOpen(false)} className="px-4 py-2 text-sm text-zinc-400 hover:text-white font-medium">Cancel</button>
                     <button type="submit" disabled={generating} className="bg-v-accent text-v-bg px-6 py-2 rounded-md font-bold text-sm hover:brightness-110 flex items-center shadow-lg shadow-v-accent/10">
                        {generating && <Loader2 className="w-3 h-3 animate-spin mr-2" />}
                        Generate
                     </button>
                  </div>
                </form>
             </motion.div>
           </div>
        )}

        {revokeKeyTarget && (
           <div className="fixed inset-0 z-[100] flex items-center justify-center p-4">
             <motion.div 
               initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
               className="absolute inset-0 bg-black/80 backdrop-blur-md"
               onClick={() => setRevokeKeyTarget(null)}
             />
             <motion.div 
               initial={{ scale: 0.9, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} exit={{ scale: 0.9, opacity: 0 }}
               className="relative bg-[#110505] border border-red-500/20 p-6 rounded-xl w-full max-w-sm shadow-[0_0_50px_rgba(239,68,68,0.1)]"
             >
                <div className="flex justify-between items-center mb-4">
                  <h3 className="text-lg font-medium text-red-500 flex items-center"><ShieldAlert className="w-5 h-5 mr-2" /> Revoke Key</h3>
                </div>
                <p className="text-sm text-red-200/60 mb-6 leading-relaxed font-sans">
                  Are you absolutely sure you want to revoke this key? Any active agent sessions utilizing this key will be terminated immediately.
                </p>
                {revokeError && <div className="p-3 mb-4 bg-red-500/20 border border-red-500/50 rounded-lg text-sm text-red-200">{revokeError}</div>}
                <div className="flex justify-end gap-3">
                   <button type="button" onClick={() => setRevokeKeyTarget(null)} className="px-4 py-2 text-sm text-red-200/50 hover:text-white font-medium">Cancel</button>
                   <button onClick={executeRevoke} className="bg-red-500 text-white px-4 py-2 rounded-md font-bold text-sm hover:bg-red-600 transition-colors shadow-lg shadow-red-500/20">
                      Revoke Key
                   </button>
                </div>
             </motion.div>
           </div>
        )}
      </AnimatePresence>

    </motion.div>
  );
}
