import React, { useState } from 'react';
import { 
  Database, 
  Key, 
  Activity, 
  Layers, 
  Clock, 
  Copy, 
  Check, 
  ArrowRight, 
  Zap,
  BarChart3
} from 'lucide-react';
import type { User, UsageStats } from './lib/types';

interface DashboardProps {
  user: User;
  stats: UsageStats;
  onOpenVisualizer: () => void;
}

export function Dashboard({ user, stats, onOpenVisualizer }: DashboardProps) {
  const [copied, setCopied] = useState(false);

  const copyKey = () => {
    navigator.clipboard.writeText(user.apiKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="p-8 md:p-12 max-w-7xl mx-auto space-y-12">
      {/* Welcome Header */}
      <div className="flex flex-col md:flex-row justify-between items-start md:items-end gap-6">
        <div>
          <h2 className="text-[10px] font-bold text-indigo-500 uppercase tracking-[0.3em] mb-3">Orchestrator Overview</h2>
          <h1 className="text-4xl font-bold text-white tracking-tight">Welcome back, {user.name.split(' ')[0]}</h1>
        </div>
        <button 
          onClick={onOpenVisualizer}
          className="px-6 py-3 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl font-bold transition-all shadow-lg shadow-indigo-500/20 flex items-center gap-2 text-sm"
        >
          Launch Visualizer <Zap className="w-4 h-4 fill-current" />
        </button>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <StatCard 
          icon={Layers} 
          label="Active Sessions" 
          value={stats.activeSessions.toString()} 
          sub="Isolated context containers"
        />
        <StatCard 
          icon={Database} 
          label="Total Facts" 
          value={stats.totalFacts.toLocaleString()} 
          sub="Stored epistemic state"
        />
        <StatCard 
          icon={BarChart3} 
          label="API Requests" 
          value={stats.totalRequests.toLocaleString()} 
          sub="Last 24 hours"
        />
      </div>

      {/* API Key Section */}
      <div className="bg-[#0a0f1e] border border-white/5 rounded-[2rem] p-10 relative overflow-hidden">
        <div className="absolute top-0 right-0 p-12 opacity-[0.03] pointer-events-none">
          <Key className="w-64 h-64 rotate-12" />
        </div>
        
        <div className="relative z-10 space-y-6">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-indigo-600/10 rounded-xl flex items-center justify-center">
              <Key className="w-5 h-5 text-indigo-500" />
            </div>
            <h3 className="text-xl font-bold text-white tracking-tight">Your API Access Token</h3>
          </div>
          
          <p className="text-slate-400 text-sm max-w-xl leading-relaxed">
            Use this key to authenticate your agent sessions. Keep it secure—anyone with this token can read and modify your epistemic graphs.
          </p>

          <div className="flex items-center gap-3 bg-[#020617] border border-white/5 p-2 pl-6 rounded-2xl max-w-2xl group">
            <code className="flex-1 font-mono text-sm text-indigo-400 truncate">
              {user.apiKey}
            </code>
            <button 
              onClick={copyKey}
              className="p-3 bg-white/5 hover:bg-white/10 rounded-xl transition-all text-slate-400 hover:text-white flex items-center gap-2 text-xs font-bold"
            >
              {copied ? <Check className="w-4 h-4 text-emerald-500" /> : <Copy className="w-4 h-4" />}
              {copied ? 'Copied' : 'Copy'}
            </button>
          </div>
        </div>
      </div>

      {/* Activity Feed / Placeholder */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <div className="space-y-6">
          <h3 className="text-lg font-bold text-white tracking-tight flex items-center gap-2">
            <Activity className="w-4 h-4 text-indigo-500" /> Recent Activity
          </h3>
          <div className="space-y-3">
            {[
              { type: 'assert', msg: 'New fact asserted in session default-session', time: '2m ago' },
              { type: 'invalidate', msg: 'Root invalidated: user_logged_out', time: '15m ago' },
              { type: 'config', msg: 'Schema updated for session prod-agent-1', time: '1h ago' }
            ].map((item, i) => (
              <div key={i} className="flex items-center justify-between p-4 bg-white/[0.02] border border-white/5 rounded-2xl">
                <div className="flex items-center gap-4">
                  <div className={`w-1.5 h-1.5 rounded-full ${item.type === 'assert' ? 'bg-indigo-500' : item.type === 'invalidate' ? 'bg-red-500' : 'bg-amber-500'}`} />
                  <span className="text-sm text-slate-300 font-medium">{item.msg}</span>
                </div>
                <span className="text-[10px] font-mono text-slate-600 uppercase">{item.time}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="space-y-6">
          <h3 className="text-lg font-bold text-white tracking-tight flex items-center gap-2">
            <Layers className="w-4 h-4 text-indigo-500" /> Resources
          </h3>
          <div className="grid grid-cols-1 gap-3">
            <ResourceLink title="SDK Reference" desc="Explore all methods for the Python & TS SDKs." />
            <ResourceLink title="Causal Logic 101" desc="Learn how to structure your reasoning sessions." />
            <ResourceLink title="Security Guide" desc="Best practices for token and session management." />
          </div>
        </div>
      </div>
    </div>
  );
}

function StatCard({ icon: Icon, label, value, sub }: any) {
  return (
    <div className="p-8 bg-white/[0.02] border border-white/5 rounded-[2rem] space-y-4">
      <div className="w-10 h-10 bg-indigo-600/10 rounded-xl flex items-center justify-center">
        <Icon className="w-5 h-5 text-indigo-500" />
      </div>
      <div>
        <div className="text-sm font-bold text-slate-500 uppercase tracking-widest mb-1">{label}</div>
        <div className="text-3xl font-bold text-white tracking-tight font-mono">{value}</div>
      </div>
      <p className="text-[10px] text-slate-600 font-bold uppercase tracking-wider">{sub}</p>
    </div>
  );
}

function ResourceLink({ title, desc }: any) {
  return (
    <div className="p-5 bg-white/[0.02] border border-white/5 rounded-2xl group hover:bg-white/[0.04] hover:border-white/10 transition-all cursor-pointer flex items-center justify-between">
      <div>
        <h4 className="text-sm font-bold text-white mb-1 group-hover:text-indigo-400 transition-colors">{title}</h4>
        <p className="text-[11px] text-slate-500">{desc}</p>
      </div>
      <ArrowRight className="w-4 h-4 text-slate-700 group-hover:text-indigo-500 group-hover:translate-x-1 transition-all" />
    </div>
  );
}
