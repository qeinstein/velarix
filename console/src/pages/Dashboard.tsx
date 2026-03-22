import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { Activity, GitMerge, ShieldCheck, Zap, Loader2 } from 'lucide-react';
import BorderGlow from '../components/reactbits/BorderGlow';
import LetterGlitch from '../components/reactbits/LetterGlitch';

export default function Dashboard() {
  const [stats, setStats] = useState([
    { label: 'Active Sessions', value: '...', change: '', icon: Activity, color: 'text-v-accent' },
    { label: 'Total Assertions', value: '...', change: '', icon: GitMerge, color: 'text-purple-400' },
    { label: 'Facts Pruned', value: '...', change: '', icon: ShieldCheck, color: 'text-v-success' },
    { label: 'API Requests', value: '...', change: '', icon: Zap, color: 'text-yellow-400' },
  ]);
  const [loading, setLoading] = useState(true);

  const [recentActivity, setRecentActivity] = useState<any[]>([]);

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const token = localStorage.getItem('velarix_token');
        const [usageRes, sessionsRes] = await Promise.all([
          fetch('http://localhost:8080/v1/org/usage', { headers: { 'Authorization': `Bearer ${token}` } }),
          fetch('http://localhost:8080/v1/sessions', { headers: { 'Authorization': `Bearer ${token}` } })
        ]);

        if (!usageRes.ok || !sessionsRes.ok) throw new Error('Failed to fetch dashboard data');
        
        const usage = await usageRes.json();
        const sessions = await sessionsRes.json();

        setStats([
          { label: 'Active Sessions', value: (sessions?.length || 0).toLocaleString(), change: '', icon: Activity, color: 'text-v-accent' },
          { label: 'Total Assertions', value: (usage?.facts_asserted || 0).toLocaleString(), change: '', icon: GitMerge, color: 'text-purple-400' },
          { label: 'Facts Pruned', value: (usage?.facts_pruned || 0).toLocaleString(), change: '', icon: ShieldCheck, color: 'text-v-success' },
          { label: 'API Requests', value: (usage?.api_requests || 0).toLocaleString(), change: '', icon: Zap, color: 'text-yellow-400' },
        ]);

        if (sessions && Array.isArray(sessions)) {
          const mappedActivity = sessions.slice(0, 5).map((s: any) => ({
            id: s.id,
            action: 'Session Created',
            agent: `Tenant: ${s.tenant_id}`,
            time: new Date(s.created_at || Date.now()).toLocaleTimeString(),
            status: 'success'
          }));
          if (mappedActivity.length > 0) {
            setRecentActivity(mappedActivity);
          }
        }
      } catch (err) {
        console.error(err);
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
  }, []);

  return (
    <div className="max-w-6xl mx-auto space-y-8 animate-in fade-in duration-500">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-display font-semibold text-white">System Overview</h1>
          <p className="text-sm text-v-text-muted mt-1">Real-time metrics for your Epistemic Engine.</p>
        </div>
        <button className="bg-white text-black px-4 py-2 rounded-md font-medium text-sm hover:bg-gray-200 transition-colors shadow-sm">
          Download Report
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {stats.map((stat, idx) => {
          const cardContent = (
            <div className="glass-panel p-5 rounded-xl flex flex-col h-full">
              <div className="flex items-center justify-between mb-4">
                <stat.icon className={`w-5 h-5 ${stat.color}`} />
                <span className={`text-xs font-medium ${stat.change.startsWith('+') ? 'text-v-success' : stat.change.startsWith('-') ? 'text-v-accent' : 'text-v-text-muted'}`}>
                  {stat.change}
                </span>
              </div>
              <div className="text-3xl font-display font-semibold text-white mb-1">
                {loading ? <Loader2 className="w-6 h-6 animate-spin" /> : stat.value}
              </div>
              <div className="text-sm text-v-text-muted font-medium">{stat.label}</div>
            </div>
          );

          // Apply glow to the first two cards for emphasis
          const isGlowing = idx === 0 || idx === 1;
          const motionContent = (
            <motion.div 
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.1 }}
              key={stat.label} 
              className="h-full"
            >
              {isGlowing ? (
                <BorderGlow 
                  config={{
                    glowColor: idx === 0 ? "rgba(6, 182, 212, 0.4)" : "rgba(167, 139, 250, 0.4)",
                    className: "h-full rounded-xl"
                  }}
                >
                  {cardContent}
                </BorderGlow>
              ) : (
                cardContent
              )}
            </motion.div>
          );

          return motionContent;
        })}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Chart placeholder */}
        <div className="lg:col-span-2 glass-panel rounded-xl p-6 min-h-[400px] flex flex-col relative overflow-hidden">
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-lg font-medium text-white">Inference Propagation Flow</h2>
            <div className="flex items-center space-x-2 text-xs text-v-text-muted">
              <span className="flex h-2 w-2 rounded-full bg-v-success"></span>
              <span>Live Engine State</span>
            </div>
          </div>
          <div className="flex-1 border border-dashed border-v-border rounded-lg flex items-center justify-center bg-v-bg/30 relative overflow-hidden">
             {/* Simulated graph lines and pulsing nodes */}
             <div className="absolute w-full h-full inset-0 opacity-20 pointer-events-none" style={{ backgroundImage: 'radial-gradient(circle at 2px 2px, #fff 1px, transparent 0)', backgroundSize: '24px 24px' }}></div>
             
             <div className="absolute inset-0 z-0 opacity-15 pointer-events-none mix-blend-screen">
                <LetterGlitch config={{ glitchColors: ['#06b6d4', '#10b981', '#3b82f6'], glitchSpeed: 40, smooth: true }} />
             </div>

             <motion.div 
               animate={{ scale: [1, 1.2, 1], opacity: [0.5, 1, 0.5] }} 
               transition={{ duration: 3, repeat: Infinity }}
               className="w-4 h-4 rounded-full bg-v-accent absolute top-1/4 left-1/4 z-10 shadow-[0_0_15px_rgba(6,182,212,0.8)]"
             ></motion.div>
             <motion.div 
               animate={{ scale: [1, 1.3, 1], opacity: [0.4, 0.8, 0.4] }} 
               transition={{ duration: 4, repeat: Infinity, delay: 1 }}
               className="w-3 h-3 rounded-full bg-v-success absolute bottom-1/3 right-1/4 z-10 shadow-[0_0_15px_rgba(16,185,129,0.8)]"
             ></motion.div>
             <div className="text-v-text-muted font-mono text-sm z-10 bg-v-bg-card/80 backdrop-blur p-3 rounded border border-v-border flex flex-col items-center space-y-2">
                {loading ? (
                  <span>Initializing engine matrix...</span>
                ) : (
                  <>
                    <span className="text-white">Active Neural Graph</span>
                    <span>Monitoring <strong className="text-v-accent">{stats[1].value}</strong> fact nodes across <strong className="text-v-success">{stats[0].value}</strong> sessions</span>
                  </>
                )}
             </div>
          </div>
        </div>

        {/* Recent Activity */}
        <div className="glass-panel rounded-xl p-6">
           <h2 className="text-lg font-medium text-white mb-6">Recent Activity</h2>
           <div className="space-y-6">
             {recentActivity.length > 0 ? (
               recentActivity.map((act) => (
                 <div key={act.id} className="flex items-start">
                   <div className={`mt-1.5 w-2 h-2 rounded-full mr-4 shrink-0 ${act.status === 'success' ? 'bg-v-success shadow-[0_0_8px_rgba(16,185,129,0.5)]' : act.status === 'error' ? 'bg-v-accent shadow-[0_0_8px_rgba(6,182,212,0.5)]' : 'bg-v-text-muted'}`}></div>
                   <div>
                     <p className="text-sm font-medium text-white">{act.action}</p>
                     <p className="text-xs text-v-text-muted mt-1">{act.agent}</p>
                   </div>
                   <div className="ml-auto text-xs text-v-text-muted font-mono">{act.time}</div>
                 </div>
               ))
             ) : (
               <div className="text-center py-8 text-sm text-v-text-muted">
                 No recent activity found.
               </div>
             )}
           </div>
           <button className="w-full mt-6 py-2 border border-v-border rounded-md text-sm text-v-text-muted hover:text-white hover:bg-white/5 transition-colors">
             View Audit Log
           </button>
        </div>
      </div>
    </div>
  );
}
