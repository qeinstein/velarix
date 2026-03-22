import React from 'react';
import { motion } from 'framer-motion';
import { Link, useNavigate } from 'react-router-dom';
import LetterGlitch from '../components/reactbits/LetterGlitch';

export default function Login() {
  const navigate = useNavigate();
  const [email, setEmail] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState('');

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await fetch('http://localhost:8080/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password })
      });
      if (!res.ok) throw new Error(await res.text() || 'Login failed');
      const data = await res.json();
      localStorage.setItem('velarix_token', data.token);
      navigate('/dashboard');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-v-bg flex flex-col justify-center items-center relative overflow-hidden font-sans">
       <div className="absolute inset-0 z-0 opacity-30 pointer-events-none mix-blend-screen">
          <LetterGlitch 
            config={{
              glitchColors: ['#06b6d4', '#4ade80', '#3b82f6'],
              glitchSpeed: 30,
              smooth: true
            }}
          />
       </div>
       <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[800px] h-[800px] bg-v-accent/5 rounded-full filter blur-[100px] pointer-events-none"></div>
       
       <Link to="/" className="absolute top-8 left-8 flex items-center space-x-2 text-v-text-muted hover:text-white transition-colors">
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"></path></svg>
          <span className="text-sm font-medium">Back to site</span>
       </Link>

       <motion.div 
         initial="hidden"
         animate="visible"
         variants={{ hidden: { opacity: 0, scale: 0.95, y: 20 }, visible: { opacity: 1, scale: 1, y: 0, transition: { staggerChildren: 0.1, duration: 0.5 } } }}
         className="glass-panel p-8 sm:p-10 rounded-2xl w-full max-w-md z-10"
       >
          <motion.div variants={{ hidden: { opacity: 0, y: 15 }, visible: { opacity: 1, y: 0 } }} className="text-center mb-8">
            <div className="w-12 h-12 mx-auto rounded-xl bg-gradient-to-br from-v-accent to-v-success flex items-center justify-center mb-4 shadow-[0_0_20px_rgba(6,182,212,0.3)]">
              <span className="text-v-bg font-bold font-display text-2xl">V</span>
            </div>
            <h2 className="text-2xl font-display font-semibold text-white">Sign in to Velarix</h2>
            <p className="text-sm text-v-text-muted mt-2">Enter your credentials to access the console</p>
          </motion.div>

          <form onSubmit={handleLogin} className="space-y-4">
            {error && <motion.div variants={{ hidden: { opacity: 0 }, visible: { opacity: 1 } }} className="p-3 bg-red-500/20 border border-red-500/50 rounded-lg text-sm text-red-200">{error}</motion.div>}
            <motion.div variants={{ hidden: { opacity: 0, y: 10 }, visible: { opacity: 1, y: 0 } }}>
              <label className="block text-xs font-medium text-v-text-muted mb-1 ml-1 uppercase">Email</label>
              <input type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder="agent@org.com" className="w-full bg-black/50 border border-v-border rounded-lg px-4 py-2.5 text-white placeholder:text-zinc-600 focus:outline-none focus:border-v-accent focus:ring-1 focus:ring-v-accent transition-all" required />
            </motion.div>
            <motion.div variants={{ hidden: { opacity: 0, y: 10 }, visible: { opacity: 1, y: 0 } }}>
              <div className="flex items-center justify-between mb-1 ml-1">
                 <label className="block text-xs font-medium text-v-text-muted uppercase">Password</label>
                 <a href="#" className="text-xs text-v-accent hover:text-v-accent/80 transition-colors">Forgot?</a>
              </div>
              <input type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="••••••••" className="w-full bg-black/50 border border-v-border rounded-lg px-4 py-2.5 text-white placeholder:text-zinc-600 focus:outline-none focus:border-v-accent focus:ring-1 focus:ring-v-accent transition-all" required />
            </motion.div>
            
            <motion.button variants={{ hidden: { opacity: 0, y: 10 }, visible: { opacity: 1, y: 0 } }} type="submit" disabled={loading} className="w-full bg-white text-black font-medium py-2.5 rounded-lg mt-6 hover:bg-gray-200 transition-colors shadow-[0_0_15px_rgba(255,255,255,0.1)] disabled:opacity-50">
              {loading ? 'Signing In...' : 'Sign In'}
            </motion.button>
          </form>

          <motion.p variants={{ hidden: { opacity: 0 }, visible: { opacity: 1 } }} className="mt-8 text-center text-sm text-v-text-muted">
            Don't have an account? <Link to="/signup" className="text-white hover:text-v-accent transition-colors">Sign up</Link>
          </motion.p>
       </motion.div>
    </div>
  );
}
