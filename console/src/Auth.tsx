import { useMemo, useState } from 'react';
import {
  ArrowRight,
  Database,
  Loader2,
  Lock,
  Mail,
  ShieldCheck,
  User as UserIcon,
} from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { VelarixClient } from './lib/client';
import type { User } from './lib/types';

interface AuthProps {
  mode: 'login' | 'signup';
  onSuccess: (user: User) => void;
  onToggleMode: () => void;
  baseUrl?: string;
}

function getFallbackName(email: string) {
  const [prefix] = email.split('@');
  return prefix
    .split(/[._-]/g)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');
}

export function Auth({ mode, onSuccess, onToggleMode, baseUrl = 'http://localhost:8080/v1' }: AuthProps) {
  const client = useMemo(() => new VelarixClient(baseUrl), [baseUrl]);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const ensureApiKey = async (targetEmail: string, authToken: string) => {
    const keys = await client.listKeys(targetEmail, authToken);
    const active = keys.find((key) => !key.is_revoked);
    if (active) return active.key;
    const created = await client.generateKey(targetEmail, 'Console', authToken);
    return created.key;
  };

  const handleForgotPassword = async () => {
    if (!email) {
      setError('Enter your email first so the reset request knows where to send the token.');
      return;
    }

    setLoading(true);
    setError(null);
    setNotice(null);
    try {
      const response = await fetch(`${baseUrl}/v1/auth/reset-request`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      });
      if (!response.ok) throw new Error(await response.text());
      setNotice('If the account exists, a reset token was sent to the server log.');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Reset request failed.');
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setLoading(true);
    setError(null);
    setNotice(null);

    try {
      if (mode === 'signup') {
        await client.register(email, password);
      }

      const token = await client.login(email, password);
      const apiKey = await ensureApiKey(email, token);
      const cachedUser = localStorage.getItem('velarix_user');
      const parsedUser = cachedUser ? (JSON.parse(cachedUser) as Partial<User>) : null;
      const resolvedName = mode === 'signup'
        ? (name.trim() || getFallbackName(email))
        : parsedUser?.email === email && parsedUser.name
          ? parsedUser.name
          : getFallbackName(email);

      const user: User = {
        name: resolvedName,
        email,
        apiKey,
        defaultMode: parsedUser?.defaultMode === 'strict' ? 'strict' : 'warn',
      };

      localStorage.setItem('velarix_user', JSON.stringify(user));
      localStorage.setItem('velarix_api_key', apiKey);
      onSuccess(user);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Authentication failed.';
      setError(message.includes('failed to save user') ? 'That account already exists. Use login instead.' : message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="page-backdrop min-h-screen px-6 py-8 lg:px-8">
      <div className="mx-auto grid min-h-[calc(100vh-4rem)] max-w-7xl gap-8 lg:grid-cols-[0.92fr_1.08fr] lg:items-center">
        <motion.div 
          initial={{ opacity: 0, x: -20 }}
          animate={{ opacity: 1, x: 0 }}
          className="surface-panel-strong rounded-[2rem] p-8 md:p-10 lg:p-12 relative overflow-hidden"
        >
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(14,165,233,0.08),transparent_50%)]" />
          <div className="relative z-10">
            <div className="section-kicker">
              <ShieldCheck className="h-3.5 w-3.5" /> Access control
            </div>
            <h1 className="font-display mt-6 text-4xl font-bold tracking-[-0.04em] text-white md:text-5xl">
              {mode === 'login' ? 'Sign in to the control room.' : 'Create a real console workspace.'}
            </h1>
            <p className="mt-6 max-w-xl text-lg leading-8 text-slate-300">
              This screen now uses the actual backend routes. No more fake user objects, fake delays, or random keys that the API server has never heard of.
            </p>

            <div className="mt-10 grid gap-4 sm:grid-cols-2">
              {[
                'JWT-backed login for console access',
                'Real API key provisioning through the backend',
                'Reset flow available through the server log token',
                'Profile state persisted locally after sign-in',
              ].map((item, i) => (
                <motion.div 
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: 0.1 + i * 0.1 }}
                  key={item} 
                  className="rounded-2xl border border-white/8 bg-black/10 px-4 py-4 text-sm leading-7 text-slate-300 hover:border-white/20 transition-colors shadow-inner"
                >
                  {item}
                </motion.div>
              ))}
            </div>
          </div>
        </motion.div>

        <motion.div 
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          className="surface-panel rounded-[2rem] p-8 md:p-10 lg:p-12 shadow-2xl hover:border-white/10 transition-colors duration-500"
        >
          <div className="flex items-center gap-3">
            <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-sky-300/14 bg-sky-400/10 shadow-inner">
              <Database className="h-6 w-6 text-sky-300" />
            </div>
            <div>
              <div className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">Velarix console</div>
              <div className="font-display text-2xl font-semibold text-white">
                {mode === 'login' ? 'Welcome back' : 'Create account'}
              </div>
            </div>
          </div>

          <form onSubmit={handleSubmit} className="mt-8 space-y-6">
            <AnimatePresence mode="popLayout">
              {mode === 'signup' && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  transition={{ duration: 0.2 }}
                >
                  <Field label="Display name" icon={UserIcon}>
                    <input
                      required
                      value={name}
                      onChange={(event) => setName(event.target.value)}
                      className="input-shell px-12 py-3.5 focus:ring-sky-500/20"
                      placeholder="Ada Lovelace"
                      type="text"
                    />
                  </Field>
                </motion.div>
              )}
            </AnimatePresence>

            <Field label="Email" icon={Mail}>
              <input
                required
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                className="input-shell px-12 py-3.5 focus:ring-sky-500/20"
                placeholder="name@company.com"
                type="email"
              />
            </Field>

            <Field label="Password" icon={Lock}>
              <input
                required
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                className="input-shell px-12 py-3.5 focus:ring-sky-500/20"
                placeholder="••••••••"
                type="password"
              />
            </Field>

            <AnimatePresence mode="popLayout">
              {error && (
                <motion.div 
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -10 }}
                  className="rounded-2xl border border-rose-400/20 bg-rose-400/10 px-4 py-4 text-sm text-rose-200"
                >
                  {error}
                </motion.div>
              )}
              {notice && (
                <motion.div 
                  initial={{ opacity: 0, y: -10 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -10 }}
                  className="rounded-2xl border border-emerald-400/20 bg-emerald-400/10 px-4 py-4 text-sm text-emerald-200"
                >
                  {notice}
                </motion.div>
              )}
            </AnimatePresence>

            <motion.button 
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              disabled={loading} 
              className="button-primary w-full px-5 py-4 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-70 shadow-lg shadow-sky-500/20"
            >
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <ArrowRight className="h-4 w-4" />}
              {mode === 'login' ? 'Sign in' : 'Create account'}
            </motion.button>
          </form>

          <div className="mt-6 flex flex-col gap-4 border-t border-white/8 pt-6 text-sm text-slate-400 sm:flex-row sm:items-center sm:justify-between">
            <button onClick={onToggleMode} className="text-left font-medium text-white transition-colors hover:text-sky-300">
              {mode === 'login' ? 'Need an account? Create one.' : 'Already have an account? Sign in.'}
            </button>
            {mode === 'login' && (
              <button onClick={handleForgotPassword} className="text-left font-medium text-slate-400 transition-colors hover:text-white">
                Forgot password
              </button>
            )}
          </div>
        </motion.div>
      </div>
    </div>
  );
}

function Field({
  label,
  icon: Icon,
  children,
}: {
  label: string;
  icon: typeof Mail;
  children: React.ReactNode;
}) {
  return (
    <label className="block space-y-2">
      <span className="text-[0.72rem] uppercase tracking-[0.22em] text-slate-500">{label}</span>
      <div className="relative group">
        <Icon className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-500 group-focus-within:text-sky-400 transition-colors" />
        {children}
      </div>
    </label>
  );
}
