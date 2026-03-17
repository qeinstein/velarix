import React, { useState } from 'react';
import { Key, Mail, Copy, Check, ArrowLeft, Database, Loader2 } from 'lucide-react';

interface KeysProps {
  onBack: () => void;
  baseUrl: string;
}

export function Keys({ onBack, baseUrl }: KeysProps) {
  const [email, setEmail] = useState('');
  const [apiKey, setApiKey] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const handleGenerate = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${baseUrl}/keys/generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email })
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      setApiKey(data.api_key);
    } catch (err: any) {
      setError(err.message || 'Failed to generate key');
    } finally {
      setLoading(false);
    }
  };

  const copyToClipboard = () => {
    if (!apiKey) return;
    navigator.clipboard.writeText(apiKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-200 font-sans flex flex-col items-center p-8">
      <div className="w-full max-w-lg">
        <button 
          onClick={onBack}
          className="flex items-center gap-2 text-slate-500 hover:text-slate-200 transition-colors mb-12 text-sm font-medium"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Dashboard
        </button>

        <div className="bg-slate-900 border border-slate-800 rounded-3xl p-10 shadow-2xl">
          <div className="w-14 h-14 bg-indigo-600 rounded-2xl flex items-center justify-center shadow-lg shadow-indigo-500/20 mb-8 mx-auto">
            <Key className="w-8 h-8 text-white" />
          </div>
          
          <h1 className="text-3xl font-bold text-center mb-3">Velarix Keys</h1>
          <p className="text-slate-500 text-center mb-10 text-sm leading-relaxed px-4">
            Enter your email to receive a unique API key for the Velarix Orchestrator.
          </p>

          {!apiKey ? (
            <form onSubmit={handleGenerate} className="space-y-6">
              <div>
                <label className="text-[10px] font-bold text-slate-500 uppercase tracking-widest block mb-2 px-1">Email Address</label>
                <div className="relative">
                  <Mail className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
                  <input 
                    type="email" 
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl pl-12 pr-4 py-3.5 text-sm focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 outline-none transition-all"
                    placeholder="name@company.com"
                  />
                </div>
              </div>
              <button 
                disabled={loading}
                className="w-full py-4 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white rounded-xl text-sm font-bold transition-all shadow-lg shadow-indigo-500/20 flex items-center justify-center gap-2"
              >
                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : "Generate API Key"}
              </button>
              {error && <p className="text-xs text-red-400 text-center font-medium bg-red-400/10 py-2 rounded-lg">{error}</p>}
            </form>
          ) : (
            <div className="space-y-8 animate-in fade-in slide-in-from-bottom-4">
              <div className="p-6 bg-emerald-500/10 border border-emerald-500/20 rounded-2xl">
                <p className="text-[10px] font-bold text-emerald-500 uppercase tracking-widest mb-4 text-center">Your unique key is ready</p>
                <div className="flex items-center gap-3 bg-slate-950 p-4 rounded-xl border border-slate-800">
                  <code className="flex-1 text-indigo-400 font-mono text-sm break-all">{apiKey}</code>
                  <button 
                    onClick={copyToClipboard}
                    className="p-2 hover:bg-slate-800 rounded-lg transition-colors text-slate-400 hover:text-white"
                  >
                    {copied ? <Check className="w-4 h-4 text-emerald-500" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>
              </div>
              
              <div className="space-y-4">
                <p className="text-xs text-slate-500 text-center italic">
                  Store this key safely. You'll need it to authenticate every SDK request.
                </p>
                <button 
                  onClick={() => setApiKey(null)}
                  className="w-full py-3 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-xl text-xs font-bold transition-all"
                >
                  Generate Another Key
                </button>
              </div>
            </div>
          )}
        </div>

        <div className="mt-12 flex justify-center items-center gap-8 opacity-40 grayscale contrast-125">
           <div className="flex items-center gap-2"><Database className="w-4 h-4" /><span className="text-xs font-bold uppercase tracking-tighter">Powered by Epistemic Go</span></div>
        </div>
      </div>
    </div>
  );
}
