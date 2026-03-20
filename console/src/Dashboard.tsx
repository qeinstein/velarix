import { useState, useEffect } from 'react';
import {
  Activity,
  ArrowRight,
  Check,
  Copy,
  Database,
  GitBranch,
  Key,
  Layers,
  Shield,
  Zap,
} from 'lucide-react';
import { motion, animate } from 'framer-motion';
import type { JournalEntry, UsageStats, User } from './lib/types';

interface DashboardProps {
  user: User;
  stats: UsageStats;
  recentActivity: JournalEntry[];
  onOpenVisualizer: () => void;
  onOpenDocs: () => void;
  onOpenKeys: () => void;
}

function AnimatedCounter({ value }: { value: number }) {
  const [displayValue, setDisplayValue] = useState(0);

  useEffect(() => {
    const controls = animate(0, value, {
      duration: 1.5,
      ease: "easeOut",
      onUpdate: (v) => {
        setDisplayValue(Math.round(v));
      }
    });
    return () => controls.stop();
  }, [value]);

  return <span>{displayValue.toLocaleString()}</span>;
}

export function Dashboard({
  user,
  stats,
  recentActivity,
  onOpenVisualizer,
  onOpenDocs,
  onOpenKeys,
}: DashboardProps) {
  const [copied, setCopied] = useState(false);

  const copyKey = async () => {
    await navigator.clipboard.writeText(user.apiKey);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1800);
  };

  return (
    <div className="page-backdrop min-h-full overflow-y-auto px-6 py-8 lg:px-8">
      <div className="mx-auto max-w-7xl space-y-8">
        <section className="surface-panel-strong rounded-[2rem] p-8 md:p-10 relative overflow-hidden group">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(14,165,233,0.1),transparent_50%)] opacity-0 group-hover:opacity-100 transition-opacity duration-700" />
          <div className="grid gap-8 lg:grid-cols-[1.05fr_0.95fr] lg:items-end relative z-10">
            <div>
              <div className="section-kicker">
                <Activity className="h-3.5 w-3.5" /> Control room overview
              </div>
              <h1 className="font-display mt-6 text-4xl font-bold tracking-[-0.04em] text-white md:text-5xl">
                Welcome back, {user.name.split(' ')[0]}.
              </h1>
              <p className="mt-5 max-w-2xl text-lg leading-8 text-slate-300">
                The dashboard should tell you whether the system is usable, trustworthy, and ready for inspection. This version stops pretending with fake telemetry and centers the data the console actually has.
              </p>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <motion.button 
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
                onClick={onOpenVisualizer} 
                className="button-primary px-5 py-4 text-sm font-semibold shadow-xl shadow-sky-500/10"
              >
                Launch visualizer <Zap className="h-4 w-4" />
              </motion.button>
              <motion.button 
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
                onClick={onOpenDocs} 
                className="button-secondary px-5 py-4 text-sm font-semibold shadow-xl shadow-black/20"
              >
                Read docs
              </motion.button>
            </div>
          </div>
        </section>

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <StatCard icon={Layers} label="Active sessions" value={stats.activeSessions} detail="Sessions currently visible to the console" delay={0} />
          <StatCard icon={Database} label="Facts in memory" value={stats.totalFacts} detail="Facts loaded for the active session" delay={0.1} />
          <StatCard icon={Activity} label="Journal events" value={stats.journalEvents} detail="Replayed history available for inspection" delay={0.2} />
          <StatCard icon={Shield} label="Policy warnings" value={stats.violationCount} detail="Facts carrying validation issues or collapses" delay={0.3} />
        </section>

        <section className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
          <motion.div 
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.4 }}
            className="surface-panel rounded-[2rem] p-7 md:p-8 hover:border-white/10 transition-colors duration-500"
          >
            <div className="flex items-center justify-between gap-4">
              <div>
                <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">API access token</div>
                <div className="mt-2 font-display text-2xl font-semibold text-white">Primary workspace key</div>
              </div>
              <motion.button 
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
                onClick={onOpenKeys} 
                className="button-secondary px-4 py-3 text-sm font-semibold"
              >
                Manage keys <ArrowRight className="h-4 w-4" />
              </motion.button>
            </div>

            <div className="mt-6 rounded-[1.6rem] border border-white/10 bg-[#050a14] p-3 shadow-inner">
              <div className="flex flex-col gap-3 md:flex-row md:items-center">
                <code className="flex-1 overflow-hidden text-ellipsis whitespace-nowrap px-3 py-3 text-sm text-sky-200">
                  {user.apiKey}
                </code>
                <motion.button 
                  whileHover={{ scale: 1.02 }}
                  whileTap={{ scale: 0.98 }}
                  onClick={copyKey} 
                  className="button-primary min-w-[132px] px-4 py-3 text-sm font-semibold"
                >
                  {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  {copied ? 'Copied' : 'Copy key'}
                </motion.button>
              </div>
            </div>

            <div className="mt-6 grid gap-4 md:grid-cols-3">
              {[
                { label: 'Default mode', value: user.defaultMode.toUpperCase() },
                { label: 'Identity', value: user.email },
                { label: 'Console state', value: 'Connected profile' },
              ].map((item, i) => (
                <motion.div 
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.5 + i * 0.1 }}
                  key={item.label} 
                  className="rounded-2xl border border-white/8 bg-white/[0.03] p-4 hover:bg-white/[0.05] transition-colors"
                >
                  <div className="text-[0.68rem] uppercase tracking-[0.18em] text-slate-500">{item.label}</div>
                  <div className="mt-2 text-sm font-medium text-slate-200">{item.value}</div>
                </motion.div>
              ))}
            </div>
          </motion.div>

          <motion.div 
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.5 }}
            className="surface-panel rounded-[2rem] p-7 md:p-8 hover:border-white/10 transition-colors duration-500"
          >
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-sky-300/12 bg-sky-400/10">
                <GitBranch className="h-5 w-5 text-sky-300" />
              </div>
              <div>
                <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Console priorities</div>
                <div className="font-display text-2xl font-semibold text-white">What to check next</div>
              </div>
            </div>

            <div className="mt-6 space-y-3 text-sm text-slate-300">
              {[
                'Use the visualizer first if you need to understand the current belief graph.',
                'Use key management before production tests so you are not running the console on a stale token.',
                'Use docs to confirm the real API surface instead of trusting the marketing copy.',
              ].map((item, i) => (
                <motion.div 
                  initial={{ opacity: 0, x: 20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: 0.6 + i * 0.1 }}
                  key={item} 
                  className="rounded-2xl border border-white/8 bg-black/10 px-4 py-4 leading-7 hover:border-white/20 transition-colors shadow-inner"
                >
                  {item}
                </motion.div>
              ))}
            </div>
          </motion.div>
        </section>

        <section className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
          <motion.div 
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.6 }}
            className="surface-panel rounded-[2rem] p-7 md:p-8 hover:border-white/10 transition-colors duration-500"
          >
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-sky-300/12 bg-sky-400/10">
                <Activity className="h-5 w-5 text-sky-300" />
              </div>
              <div>
                <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Recent journal activity</div>
                <div className="font-display text-2xl font-semibold text-white">Session changes</div>
              </div>
            </div>

            <div className="mt-6 space-y-3">
              {recentActivity.length > 0 ? (
                recentActivity.map((entry, index) => (
                  <motion.div 
                    initial={{ opacity: 0, scale: 0.98 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: 0.7 + index * 0.05 }}
                    key={`${entry.timestamp}-${index}`} 
                    className="group flex items-center justify-between gap-4 rounded-2xl border border-white/8 bg-black/10 px-4 py-4 hover:bg-white/[0.03] transition-colors"
                  >
                    <div className="flex items-center gap-3">
                      <div className={`h-2.5 w-2.5 rounded-full ${entry.type === 'invalidate' ? 'bg-rose-400 shadow-[0_0_8px_rgba(251,113,133,0.8)]' : entry.type === 'confidence_adjusted' ? 'bg-amber-300 shadow-[0_0_8px_rgba(252,211,77,0.8)]' : 'bg-sky-300 shadow-[0_0_8px_rgba(56,189,248,0.8)]'}`} />
                      <div>
                        <div className="text-sm font-medium text-slate-200 group-hover:text-white transition-colors">{entry.fact_id || entry.fact?.ID || 'Session event'}</div>
                        <div className="mt-1 text-xs uppercase tracking-[0.16em] text-slate-500">{entry.type.replaceAll('_', ' ')}</div>
                      </div>
                    </div>
                    <div className="text-[0.68rem] uppercase tracking-[0.18em] text-slate-500 bg-white/5 px-2.5 py-1 rounded-md">
                      {new Date(entry.timestamp).toLocaleTimeString()}
                    </div>
                  </motion.div>
                ))
              ) : (
                <EmptyState
                  title="No journal events yet"
                  description="Start asserting facts or loading a populated session to see meaningful activity here."
                />
              )}
            </div>
          </motion.div>

          <motion.div 
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.7 }}
            className="surface-panel rounded-[2rem] p-7 md:p-8 hover:border-white/10 transition-colors duration-500"
          >
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-sky-300/12 bg-sky-400/10">
                <Key className="h-5 w-5 text-sky-300" />
              </div>
              <div>
                <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Readiness check</div>
                <div className="font-display text-2xl font-semibold text-white">Product posture</div>
              </div>
            </div>

            <div className="mt-6 space-y-3">
              {[
                {
                  title: 'Authenticated workspace',
                  status: Boolean(user.apiKey),
                  note: 'The console now has a real API key attached to the user profile.',
                },
                {
                  title: 'Inspectable session history',
                  status: stats.journalEvents > 0,
                  note: 'Journal events exist when the session has already observed or invalidated facts.',
                },
                {
                  title: 'Validation awareness',
                  status: true,
                  note: 'Warnings are surfaced as a first-class dashboard metric instead of hidden in the graph view.',
                },
              ].map((item, index) => (
                <motion.div 
                  initial={{ opacity: 0, x: 20 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: 0.8 + index * 0.1 }}
                  key={item.title} 
                  className="rounded-2xl border border-white/8 bg-black/10 px-4 py-4 hover:border-white/15 transition-colors"
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="text-sm font-medium text-slate-200">{item.title}</div>
                    <div className={`rounded-full px-3 py-1 text-[0.65rem] font-semibold uppercase tracking-[0.18em] shadow-inner ${item.status ? 'border border-emerald-400/25 bg-emerald-400/10 text-emerald-300' : 'border border-amber-300/25 bg-amber-300/10 text-amber-200'}`}>
                      {item.status ? 'Ready' : 'Pending'}
                    </div>
                  </div>
                  <p className="mt-3 text-sm leading-7 text-slate-400">{item.note}</p>
                </motion.div>
              ))}
            </div>
          </motion.div>
        </section>
      </div>
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  detail,
  delay
}: {
  icon: typeof Database;
  label: string;
  value: number;
  detail: string;
  delay: number;
}) {
  return (
    <motion.div 
      initial={{ opacity: 0, scale: 0.95, y: 20 }}
      animate={{ opacity: 1, scale: 1, y: 0 }}
      transition={{ delay, duration: 0.4, ease: "easeOut" }}
      whileHover={{ y: -4, scale: 1.02 }}
      className="surface-panel rounded-[1.7rem] p-6 hover:shadow-[0_20px_40px_rgba(0,0,0,0.4)] transition-shadow duration-300"
    >
      <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-sky-300/12 bg-sky-400/10 shadow-inner">
        <Icon className="h-5 w-5 text-sky-300" />
      </div>
      <div className="mt-6 text-[0.72rem] uppercase tracking-[0.2em] text-slate-500 font-bold">{label}</div>
      <div className="font-display mt-2 text-4xl font-semibold text-white drop-shadow-md">
        <AnimatedCounter value={value} />
      </div>
      <p className="mt-3 text-sm leading-7 text-slate-400">{detail}</p>
    </motion.div>
  );
}

function EmptyState({ title, description }: { title: string; description: string }) {
  return (
    <motion.div 
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      className="rounded-[1.7rem] border border-dashed border-white/12 bg-black/10 px-5 py-8 text-center"
    >
      <div className="font-display text-2xl font-semibold text-white">{title}</div>
      <p className="mt-3 text-sm leading-7 text-slate-400">{description}</p>
    </motion.div>
  );
}
