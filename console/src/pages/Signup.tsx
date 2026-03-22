import React from 'react';
import { motion } from 'framer-motion';
import { Link, useNavigate } from 'react-router-dom';
import LetterGlitch from '../components/reactbits/LetterGlitch';

export default function Signup() {
  const navigate = useNavigate();
  const [email, setEmail] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [orgName, setOrgName] = React.useState('');
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState('');

  const handleSignup = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await fetch('http://localhost:8080/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password })
      });
      if (!res.ok) throw new Error(await res.text() || 'Registration failed');
      
      const loginRes = await fetch('http://localhost:8080/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password })
      });
      if (!loginRes.ok) throw new Error('Login failed after registration');
      const data = await loginRes.json();
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
              glitchColors: ['#10b981', '#06b6d4', '#3b82f6'],
              glitchSpeed: 30,
              smooth: true
            }}
          />
       </div>
       <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[800px] h-[800px] bg-v-success/5 rounded-full filter blur-[100px] pointer-events-none"></div>
       
       <Link to="/" className="absolute top-8 left-8 flex items-center space-x-2 text-v-text-muted hover:text-white transition-colors">
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10 19l-7-7m0 0l7-7m-7 7h18"></path></svg>
          <span className="text-sm font-medium">Back to site</span>
       </Link>

       <motion.div 
         initial={{ opacity: 0, y: 20 }}
         animate={{ opacity: 1, y: 0 }}
         className="glass-panel p-8 sm:p-10 rounded-2xl w-full max-w-md z-10"
       >
          <div className="text-center mb-8">
            <div className="w-12 h-12 mx-auto rounded-xl bg-gradient-to-br from-v-success to-v-accent flex items-center justify-center mb-4 shadow-[0_0_20px_rgba(16,185,129,0.3)]">
              <span className="text-v-bg font-bold font-display text-2xl">V</span>
            </div>
            <h2 className="text-2xl font-display font-semibold text-white">Initialize Tenant</h2>
            <p className="text-sm text-v-text-muted mt-2">Create a new organization in the Epistemic Engine</p>
          </div>

          <form onSubmit={handleSignup} className="space-y-4">
            {error && <div className="p-3 bg-red-500/20 border border-red-500/50 rounded-lg text-sm text-red-200">{error}</div>}
            <div>
              <label className="block text-xs font-medium text-v-text-muted mb-1 ml-1 uppercase">Organization Name</label>
              <input type="text" value={orgName} onChange={e => setOrgName(e.target.value)} placeholder="Acme Healthcare Agent" className="w-full bg-black/50 border border-v-border rounded-lg px-4 py-2.5 text-white placeholder:text-zinc-600 focus:outline-none focus:border-v-success focus:ring-1 focus:ring-v-success transition-all" required />
            </div>
            <div>
              <label className="block text-xs font-medium text-v-text-muted mb-1 ml-1 uppercase">Work Email</label>
              <input type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder="admin@acme.com" className="w-full bg-black/50 border border-v-border rounded-lg px-4 py-2.5 text-white placeholder:text-zinc-600 focus:outline-none focus:border-v-success focus:ring-1 focus:ring-v-success transition-all" required />
            </div>
            <div>
              <label className="block text-xs font-medium text-v-text-muted mb-1 ml-1 uppercase">Master Password</label>
              <input type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="••••••••" className="w-full bg-black/50 border border-v-border rounded-lg px-4 py-2.5 text-white placeholder:text-zinc-600 focus:outline-none focus:border-v-success focus:ring-1 focus:ring-v-success transition-all" required />
            </div>
            
            <button type="submit" disabled={loading} className="w-full bg-white text-black font-medium py-2.5 rounded-lg mt-6 hover:bg-gray-200 transition-colors shadow-[0_0_15px_rgba(255,255,255,0.1)] disabled:opacity-50">
              {loading ? 'Creating...' : 'Create Organization'}
            </button>
          </form>

          <p className="mt-8 text-center text-sm text-v-text-muted">
            Already registered? <Link to="/login" className="text-white hover:text-v-success transition-colors">Sign in</Link>
          </p>
       </motion.div>
    </div>
  );
}
