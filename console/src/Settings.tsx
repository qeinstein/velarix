import { useState } from 'react';
import { ArrowRight, Check, Mail, Shield, User as UserIcon } from 'lucide-react';
import { motion } from 'framer-motion';
import type { User } from './lib/types';

interface SettingsProps {
  user: User;
  onUpdate: (user: User) => void;
  onManageKeys: () => void;
}

export function Settings({ user, onUpdate, onManageKeys }: SettingsProps) {
  const [name, setName] = useState(user.name);
  const [defaultMode, setDefaultMode] = useState<User['defaultMode']>(user.defaultMode);
  const [saved, setSaved] = useState(false);

  const handleSave = (event: React.FormEvent) => {
    event.preventDefault();
    const nextUser = { ...user, name, defaultMode };
    localStorage.setItem('velarix_user', JSON.stringify(nextUser));
    onUpdate(nextUser);
    setSaved(true);
    window.setTimeout(() => setSaved(false), 1800);
  };

  return (
    <div className="page-backdrop min-h-full overflow-y-auto px-6 py-8 lg:px-8">
      <div className="mx-auto max-w-5xl space-y-8">
        <motion.section 
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="surface-panel-strong rounded-[2rem] p-8 md:p-10 relative overflow-hidden"
        >
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(14,165,233,0.08),transparent_50%)]" />
          <div className="grid gap-6 lg:grid-cols-[1fr_0.9fr] lg:items-end relative z-10">
            <div>
              <div className="section-kicker">
                <Shield className="h-3.5 w-3.5" /> Console preferences
              </div>
              <h1 className="font-display mt-6 text-4xl font-bold tracking-[-0.04em] text-white md:text-5xl">
                Keep settings honest and operational.
              </h1>
              <p className="mt-5 max-w-2xl text-lg leading-8 text-slate-300">
                Settings should describe what the console really controls. Key rotation is handled in dedicated key management now, not by inventing a new token in local storage.
              </p>
            </div>
            <motion.button 
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              onClick={onManageKeys} 
              className="button-secondary px-5 py-4 text-sm font-semibold shadow-lg shadow-black/20"
            >
              Open key management <ArrowRight className="h-4 w-4" />
            </motion.button>
          </div>
        </motion.section>

        <form onSubmit={handleSave} className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
          <motion.section 
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.1 }}
            className="surface-panel rounded-[2rem] p-7 md:p-8 hover:border-white/10 transition-colors duration-500"
          >
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-sky-300/12 bg-sky-400/10 shadow-inner">
                <UserIcon className="h-5 w-5 text-sky-300" />
              </div>
              <div>
                <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Profile</div>
                <div className="font-display text-2xl font-semibold text-white">Console identity</div>
              </div>
            </div>

            <div className="mt-6 space-y-6">
              <label className="block space-y-2">
                <span className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Display name</span>
                <input value={name} onChange={(event) => setName(event.target.value)} className="input-shell px-4 py-3.5" />
              </label>

              <div className="rounded-[1.6rem] border border-white/8 bg-black/10 p-4 shadow-inner hover:border-white/15 transition-colors">
                <div className="flex items-center gap-3">
                  <Mail className="h-4 w-4 text-slate-500" />
                  <div>
                    <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Account email</div>
                    <div className="mt-1 text-sm text-slate-200">{user.email}</div>
                  </div>
                </div>
                <p className="mt-3 text-sm leading-7 text-slate-400">
                  Email is currently sourced from the authenticated backend account and is shown here as a reference, not a writable setting.
                </p>
              </div>
            </div>
          </motion.section>

          <motion.section 
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2 }}
            className="surface-panel rounded-[2rem] p-7 md:p-8 hover:border-white/10 transition-colors duration-500"
          >
            <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Enforcement mode</div>
            <div className="font-display mt-2 text-2xl font-semibold text-white">Default validation posture</div>
            <p className="mt-3 text-sm leading-7 text-slate-400">
              This is a console preference that controls the default mode you apply when creating or managing sessions.
            </p>

            <div className="mt-6 space-y-3">
              {[
                {
                  id: 'strict' as const,
                  title: 'Strict mode',
                  description: 'Reject invalid facts at write time and force the caller to correct them.',
                },
                {
                  id: 'warn' as const,
                  title: 'Warn mode',
                  description: 'Accept the fact, mark the warning, and surface the issue in the audit trail.',
                },
              ].map((option) => {
                const active = defaultMode === option.id;
                return (
                  <motion.button
                    whileHover={{ scale: 1.01 }}
                    whileTap={{ scale: 0.99 }}
                    key={option.id}
                    type="button"
                    onClick={() => setDefaultMode(option.id)}
                    className={`w-full rounded-[1.6rem] border px-5 py-5 text-left transition-colors shadow-sm hover:shadow-lg ${active ? 'border-sky-500/30 bg-sky-500/10 shadow-[0_4px_20px_rgba(14,165,233,0.1)]' : 'border-white/8 bg-black/10 hover:border-white/20 hover:bg-white/[0.04]'}`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className={`font-display text-xl font-semibold ${active ? 'text-white' : 'text-slate-200'}`}>{option.title}</div>
                      {active && <span className="rounded-full border border-sky-400/30 bg-sky-500/20 px-3 py-1 text-[0.65rem] uppercase tracking-[0.18em] text-sky-200 shadow-inner">Active</span>}
                    </div>
                    <p className={`mt-3 text-sm leading-7 ${active ? 'text-sky-100/70' : 'text-slate-400'}`}>{option.description}</p>
                  </motion.button>
                );
              })}
            </div>

            <motion.button 
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              type="submit" 
              className={`mt-6 w-full px-5 py-4 text-sm font-semibold shadow-lg ${saved ? 'button-primary bg-emerald-500 hover:bg-emerald-400 shadow-emerald-500/20 border-emerald-400/50' : 'button-primary shadow-sky-500/20'}`}
            >
              {saved ? <Check className="h-4 w-4" /> : <Shield className="h-4 w-4" />}
              {saved ? 'Saved' : 'Save preferences'}
            </motion.button>
          </motion.section>
        </form>
      </div>
    </div>
  );
}
