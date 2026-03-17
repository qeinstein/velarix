import React, { useState } from 'react';
import { 
  User as UserIcon, 
  Mail, 
  Shield, 
  Key, 
  RefreshCcw, 
  Save, 
  Check,
  AlertTriangle
} from 'lucide-react';
import type { User } from './lib/types';

interface SettingsProps {
  user: User;
  onUpdate: (user: User) => void;
}

export function Settings({ user, onUpdate }: SettingsProps) {
  const [name, setName] = useState(user.name);
  const [email, setEmail] = useState(user.email);
  const [defaultMode, setDefaultMode] = useState(user.defaultMode);
  const [saved, setSaved] = useState(false);

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    const updatedUser = { ...user, name, email, defaultMode };
    localStorage.setItem('velarix_user', JSON.stringify(updatedUser));
    onUpdate(updatedUser);
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  };

  const regenerateKey = () => {
    if (confirm('Are you sure? This will invalidate your existing API key across all agents.')) {
      const newKey = 'vx_' + Math.random().toString(36).substring(2, 15);
      const updatedUser = { ...user, apiKey: newKey };
      localStorage.setItem('velarix_user', JSON.stringify(updatedUser));
      onUpdate(updatedUser);
    }
  };

  return (
    <div className="p-8 md:p-12 max-w-3xl mx-auto">
      <div className="mb-12">
        <h2 className="text-[10px] font-bold text-indigo-500 uppercase tracking-[0.3em] mb-3">System Configuration</h2>
        <h1 className="text-4xl font-bold text-white tracking-tight">Settings</h1>
      </div>

      <form onSubmit={handleSave} className="space-y-12">
        {/* Profile Section */}
        <section className="space-y-6">
          <h3 className="text-lg font-bold text-white flex items-center gap-2">
            <UserIcon className="w-4 h-4 text-slate-500" /> User Profile
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-2">
              <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest px-1">Full Name</label>
              <input 
                type="text" 
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full bg-white/[0.02] border border-white/5 rounded-xl px-4 py-3 text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 outline-none transition-all"
              />
            </div>
            <div className="space-y-2">
              <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest px-1">Email Address</label>
              <input 
                type="email" 
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full bg-white/[0.02] border border-white/5 rounded-xl px-4 py-3 text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 outline-none transition-all"
              />
            </div>
          </div>
        </section>

        {/* Security Section */}
        <section className="space-y-6">
          <h3 className="text-lg font-bold text-white flex items-center gap-2">
            <Shield className="w-4 h-4 text-slate-500" /> Security & Auth
          </h3>
          <div className="p-6 bg-white/[0.02] border border-white/5 rounded-[2rem] space-y-6">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="text-sm font-bold text-white mb-1">Active API Token</h4>
                <p className="text-[11px] text-slate-500 uppercase font-mono tracking-tighter">{user.apiKey}</p>
              </div>
              <button 
                type="button"
                onClick={regenerateKey}
                className="px-4 py-2 bg-slate-800 hover:bg-red-900/30 hover:text-red-400 rounded-xl text-xs font-bold transition-all flex items-center gap-2"
              >
                <RefreshCcw className="w-3 h-3" /> Regenerate
              </button>
            </div>
          </div>
        </section>

        {/* Orchestrator Defaults */}
        <section className="space-y-6">
          <h3 className="text-lg font-bold text-white flex items-center gap-2">
            <Shield className="w-4 h-4 text-slate-500" /> Orchestrator Defaults
          </h3>
          <div className="p-1 bg-white/[0.02] border border-white/5 rounded-2xl flex">
            <button 
              type="button"
              onClick={() => setDefaultMode('strict')}
              className={`flex-1 py-3 rounded-xl text-xs font-bold transition-all flex items-center justify-center gap-2 ${
                defaultMode === 'strict' ? 'bg-white text-black' : 'text-slate-500 hover:text-slate-300'
              }`}
            >
              <Shield className="w-3.5 h-3.5" /> Strict Mode
            </button>
            <button 
              type="button"
              onClick={() => setDefaultMode('warn')}
              className={`flex-1 py-3 rounded-xl text-xs font-bold transition-all flex items-center justify-center gap-2 ${
                defaultMode === 'warn' ? 'bg-white text-black' : 'text-slate-500 hover:text-slate-300'
              }`}
            >
              <AlertTriangle className="w-3.5 h-3.5" /> Warn Mode
            </button>
          </div>
          <p className="px-2 text-[10px] text-slate-500 italic">
            * This setting defines the default enforcement mode for new reasoning sessions.
          </p>
        </section>

        <div className="pt-8 border-t border-white/5 flex justify-end">
          <button 
            type="submit"
            className="px-8 py-4 bg-indigo-600 hover:bg-indigo-500 text-white rounded-2xl font-bold transition-all shadow-xl shadow-indigo-500/20 flex items-center justify-center gap-2 text-sm"
          >
            {saved ? <Check className="w-4 h-4" /> : <Save className="w-4 h-4" />}
            {saved ? 'Changes Saved' : 'Save Configuration'}
          </button>
        </div>
      </form>
    </div>
  );
}
