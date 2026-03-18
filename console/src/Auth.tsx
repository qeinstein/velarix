import React, { useState } from 'react';
import { Database, Mail, Lock, User, ArrowRight, Loader2, ShieldCheck } from 'lucide-react';

interface AuthProps {
  mode: 'login' | 'signup';
  onSuccess: (user: any) => void;
  onToggleMode: () => void;
}

export function Auth({ mode, onSuccess, onToggleMode }: AuthProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    
    // Simulate API call
    setTimeout(() => {
      const mockUser = {
        name: mode === 'signup' ? name : 'Demo User',
        email,
        apiKey: 'vx_' + Math.random().toString(36).substring(2, 15),
        defaultMode: 'warn'
      };
      
      localStorage.setItem('velarix_user', JSON.stringify(mockUser));
      onSuccess(mockUser);
      setLoading(false);
    }, 1000);
  };

  return (
    <div className="min-h-screen bg-[#020617] text-slate-200 font-sans flex flex-col items-center justify-center p-6 selection:bg-indigo-500/30">
      <div className="w-full max-w-md">
        <div className="flex items-center justify-center gap-3 mb-12">
          <div className="w-10 h-10 bg-indigo-600 rounded-xl flex items-center justify-center shadow-lg shadow-indigo-500/20">
            <Database className="w-6 h-6 text-white" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-white">Velarix</h1>
        </div>

        <div className="bg-[#0a0f1e] border border-white/5 rounded-[2.5rem] p-10 shadow-2xl relative overflow-hidden">
          {/* Subtle Glow */}
          <div className="absolute -top-24 -right-24 w-48 h-48 bg-indigo-600/10 blur-[80px] rounded-full pointer-events-none" />
          
          <div className="relative z-10">
            <h2 className="text-3xl font-bold text-white mb-2">
              {mode === 'login' ? 'Welcome back' : 'Create account'}
            </h2>
            <p className="text-slate-500 text-sm mb-10">
              {mode === 'login' ? 'Enter your credentials to access your orchestrator.' : 'Start building with deterministic agent memory.'}
            </p>

            <form onSubmit={handleSubmit} className="space-y-6">
              {mode === 'signup' && (
                <div className="space-y-2">
                  <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest px-1">Full Name</label>
                  <div className="relative">
                    <User className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
                    <input 
                      type="text" 
                      required
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      className="w-full bg-[#020617] border border-white/5 rounded-2xl pl-12 pr-4 py-3.5 text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 outline-none transition-all placeholder:text-slate-700"
                      placeholder="Alan Turing"
                    />
                  </div>
                </div>
              )}

              <div className="space-y-2">
                <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest px-1">Email Address</label>
                <div className="relative">
                  <Mail className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
                  <input 
                    type="email" 
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="w-full bg-[#020617] border border-white/5 rounded-2xl pl-12 pr-4 py-3.5 text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 outline-none transition-all placeholder:text-slate-700"
                    placeholder="alan@velarix.dev"
                  />
                </div>
              </div>

              <div className="space-y-2">
                <div className="flex items-center justify-between px-1">
                  <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest">Password</label>
                  {mode === 'login' && <button type="button" className="text-[10px] font-bold text-indigo-500 uppercase tracking-widest hover:text-indigo-400 transition-colors">Forgot?</button>}
                </div>
                <div className="relative">
                  <Lock className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
                  <input 
                    type="password" 
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full bg-[#020617] border border-white/5 rounded-2xl pl-12 pr-4 py-3.5 text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500/50 outline-none transition-all placeholder:text-slate-700"
                    placeholder="••••••••"
                  />
                </div>
              </div>

              <button 
                disabled={loading}
                className="w-full py-4 bg-white text-black hover:bg-slate-200 disabled:opacity-50 rounded-2xl font-bold transition-all flex items-center justify-center gap-2 text-sm shadow-xl shadow-white/5"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : (
                  <>
                    {mode === 'login' ? 'Sign in' : 'Create account'}
                    <ArrowRight className="w-4 h-4" />
                  </>
                )}
              </button>
            </form>

            <div className="mt-10 pt-8 border-t border-white/5 text-center">
              <p className="text-slate-500 text-sm">
                {mode === 'login' ? "Don't have an account?" : "Already have an account?"}{' '}
                <button 
                  onClick={onToggleMode}
                  className="font-bold text-white hover:text-indigo-400 transition-colors"
                >
                  {mode === 'login' ? 'Sign up' : 'Log in'}
                </button>
              </p>
            </div>
          </div>
        </div>

        <div className="mt-12 flex items-center justify-center gap-6 opacity-30 grayscale contrast-125">
           <div className="flex items-center gap-2"><ShieldCheck className="w-4 h-4" /><span className="text-[10px] font-bold uppercase tracking-widest">Enterprise Encrypted</span></div>
        </div>
      </div>
    </div>
  );
}
