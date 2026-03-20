import { useEffect, useMemo, useState } from 'react';
import {
  ArrowLeft,
  Check,
  Copy,
  Key,
  Loader2,
  Shield,
  Trash2,
} from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { VelarixClient } from './lib/client';
import type { APIKeyRecord, User } from './lib/types';

interface KeysProps {
  onBack: () => void;
  baseUrl: string;
}

export function Keys({ onBack, baseUrl }: KeysProps) {
  const client = useMemo(() => new VelarixClient(baseUrl), [baseUrl]);
  const [user, setUser] = useState<User | null>(() => {
    const saved = localStorage.getItem('velarix_user');
    return saved ? (JSON.parse(saved) as User) : null;
  });
  const [keys, setKeys] = useState<APIKeyRecord[]>([]);
  const [label, setLabel] = useState('Console');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const loadKeys = async () => {
    if (!user?.apiKey) {
      setKeys([]);
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      const nextKeys = await client.listKeys(user.email, user.apiKey);
      setKeys(nextKeys);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load keys.');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadKeys();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.apiKey, user?.email]);

  const handleGenerate = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!user?.apiKey) return;

    try {
      setSaving(true);
      const created = await client.generateKey(user.email, label || 'Console', user.apiKey);
      const nextKeys = [created, ...keys];
      setKeys(nextKeys);
      setLabel('Console');
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate key.');
    } finally {
      setSaving(false);
    }
  };

  const handleCopy = async (value: string) => {
    await navigator.clipboard.writeText(value);
    setCopiedKey(value);
    window.setTimeout(() => setCopiedKey(null), 1800);
  };

  const handleRevoke = async (keyToRevoke: string) => {
    if (!user?.apiKey) return;

    const activeAlternatives = keys.filter((entry) => !entry.is_revoked && entry.key !== keyToRevoke);
    if (keyToRevoke === user.apiKey && activeAlternatives.length === 0) {
      setError('Generate another active key before revoking the one this console is using.');
      return;
    }

    try {
      setSaving(true);
      await client.revokeKey(user.email, keyToRevoke, user.apiKey);
      const nextKeys = keys.map((entry) => (
        entry.key === keyToRevoke ? { ...entry, is_revoked: true } : entry
      ));
      setKeys(nextKeys);

      if (keyToRevoke === user.apiKey) {
        const replacement = activeAlternatives[0];
        const nextUser = { ...user, apiKey: replacement.key };
        localStorage.setItem('velarix_user', JSON.stringify(nextUser));
        localStorage.setItem('velarix_api_key', replacement.key);
        setUser(nextUser);
      }

      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke key.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="page-backdrop min-h-screen overflow-y-auto px-6 py-8 lg:px-8">
      <div className="mx-auto max-w-6xl space-y-8">
        <motion.button 
          whileHover={{ scale: 1.02 }}
          whileTap={{ scale: 0.98 }}
          onClick={onBack} 
          className="button-secondary px-4 py-3 text-sm font-semibold shadow-lg shadow-black/20"
        >
          <ArrowLeft className="h-4 w-4" /> Back to console
        </motion.button>

        <motion.section 
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="surface-panel-strong rounded-[2rem] p-8 md:p-10 relative overflow-hidden"
        >
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(14,165,233,0.08),transparent_50%)]" />
          <div className="grid gap-6 lg:grid-cols-[1fr_0.95fr] lg:items-end relative z-10">
            <div>
              <div className="section-kicker">
                <Shield className="h-3.5 w-3.5" /> Key management
              </div>
              <h1 className="font-display mt-6 text-4xl font-bold tracking-[-0.04em] text-white md:text-5xl">
                Real API key lifecycle, not a mock textbox.
              </h1>
              <p className="mt-5 max-w-2xl text-lg leading-8 text-slate-300">
                This screen reads, creates, and revokes actual keys from the backend. The currently attached console key is highlighted so you know exactly what the UI is using.
              </p>
            </div>

            <form onSubmit={handleGenerate} className="surface-panel rounded-[1.7rem] p-5 hover:border-white/10 transition-colors duration-300">
              <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Generate new key</div>
              <div className="mt-3 flex flex-col gap-3 sm:flex-row">
                <input
                  value={label}
                  onChange={(event) => setLabel(event.target.value)}
                  className="input-shell flex-1 px-4 py-3"
                  placeholder="Production"
                />
                <motion.button 
                  whileHover={{ scale: 1.02 }}
                  whileTap={{ scale: 0.98 }}
                  disabled={saving || !user?.apiKey} 
                  className="button-primary px-5 py-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-70 shadow-lg shadow-sky-500/20"
                >
                  {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Key className="h-4 w-4" />}
                  Create key
                </motion.button>
              </div>
            </form>
          </div>
        </motion.section>

        <AnimatePresence>
          {error && (
            <motion.div 
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              className="rounded-2xl border border-rose-400/20 bg-rose-400/10 px-4 py-4 text-sm text-rose-200"
            >
              {error}
            </motion.div>
          )}
        </AnimatePresence>

        <section className="surface-panel rounded-[2rem] p-7 md:p-8">
          <div className="flex items-center gap-3 mb-6">
            <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-sky-300/12 bg-sky-400/10 shadow-inner">
              <Key className="h-5 w-5 text-sky-300" />
            </div>
            <div>
              <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Active keys</div>
              <div className="font-display text-2xl font-semibold text-white">Your workspace tokens</div>
            </div>
          </div>

          <div className="mt-6 rounded-[1.6rem] border border-white/8 bg-[#050a14] overflow-hidden">
            {loading ? (
              <div className="flex items-center justify-center p-12 text-slate-500">
                <Loader2 className="h-6 w-6 animate-spin" />
              </div>
            ) : keys.length > 0 ? (
              <table className="w-full text-left text-sm text-slate-300">
                <thead className="border-b border-white/8 bg-white/[0.02]">
                  <tr>
                    <th className="px-6 py-4 font-semibold text-white">Key</th>
                    <th className="px-6 py-4 font-semibold text-white">Label</th>
                    <th className="px-6 py-4 font-semibold text-white">Status</th>
                    <th className="px-6 py-4 font-semibold text-white">Last used</th>
                    <th className="px-6 py-4 text-right font-semibold text-white">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  <AnimatePresence>
                    {keys.map((entry) => {
                      const isActiveConsoleKey = user?.apiKey === entry.key;
                      return (
                        <motion.tr 
                          initial={{ opacity: 0, y: 10 }}
                          animate={{ opacity: 1, y: 0 }}
                          exit={{ opacity: 0, scale: 0.95 }}
                          key={entry.key} 
                          className="group border-b border-white/4 last:border-0 hover:bg-white/[0.02] transition-colors"
                        >
                          <td className="px-6 py-4">
                            <div className="flex items-center gap-3">
                              <code className={`rounded-lg px-2 py-1 text-xs ${entry.is_revoked ? 'bg-rose-400/10 text-rose-200' : 'bg-sky-400/10 text-sky-200'}`}>
                                {entry.key.slice(0, 16)}...
                              </code>
                              {!entry.is_revoked && (
                                <button
                                  onClick={() => handleCopy(entry.key)}
                                  className="text-slate-500 hover:text-white transition-colors"
                                >
                                  {copiedKey === entry.key ? <Check className="h-4 w-4 text-emerald-400" /> : <Copy className="h-4 w-4" />}
                                </button>
                              )}
                            </div>
                          </td>
                          <td className="px-6 py-4 font-medium text-slate-200">
                            <div className="flex items-center gap-2">
                              {entry.label}
                              {isActiveConsoleKey && !entry.is_revoked && (
                                <span className="rounded-full border border-sky-300/20 bg-sky-400/10 px-2 py-0.5 text-[0.6rem] uppercase tracking-[0.18em] text-sky-200">
                                  In use
                                </span>
                              )}
                            </div>
                          </td>
                          <td className="px-6 py-4">
                            <span className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[0.65rem] uppercase tracking-[0.16em] font-bold ${entry.is_revoked ? 'border-rose-400/20 bg-rose-400/10 text-rose-300' : 'border-emerald-400/20 bg-emerald-400/10 text-emerald-300'}`}>
                              {entry.is_revoked ? 'Revoked' : 'Active'}
                            </span>
                          </td>
                          <td className="px-6 py-4 text-slate-400">
                            {entry.last_used_at ? new Date(entry.last_used_at).toLocaleDateString() : 'Never'}
                          </td>
                          <td className="px-6 py-4 text-right">
                            {!entry.is_revoked && (
                              <motion.button
                                whileHover={{ scale: 1.05 }}
                                whileTap={{ scale: 0.95 }}
                                onClick={() => handleRevoke(entry.key)}
                                disabled={saving}
                                className="inline-flex items-center justify-center rounded-xl bg-rose-400/10 p-2 text-rose-400 hover:bg-rose-400 hover:text-white transition-colors disabled:opacity-50"
                              >
                                <Trash2 className="h-4 w-4" />
                              </motion.button>
                            )}
                          </td>
                        </motion.tr>
                      );
                    })}
                  </AnimatePresence>
                </tbody>
              </table>
            ) : (
              <div className="px-6 py-12 text-center text-sm text-slate-400">
                No keys found.
              </div>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
