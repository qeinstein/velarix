import { useEffect, useState, useCallback, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { RotateCcw, Loader2, ChevronDown, AlertCircle, X, ShieldAlert } from 'lucide-react';
import ForceGraph2D from 'react-force-graph-2d';

export default function NeuralGraph() {
  const [sessions, setSessions] = useState<any[]>([]);
  const [selectedSession, setSelectedSession] = useState('');
  const [graphData, setGraphData] = useState<{nodes: any[], links: any[]}>({ nodes: [], links: [] });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [selectedNode, setSelectedNode] = useState<any>(null);
  
  const graphRef = useRef<any>();

  const fetchSessions = async () => {
    try {
      const token = localStorage.getItem('velarix_token');
      const res = await fetch('http://localhost:8080/v1/sessions', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to fetch sessions');
      const data = await res.json();
      setSessions(data || []);
      if (data && data.length > 0 && !selectedSession) {
        setSelectedSession(data[0].id);
      }
    } catch (err: any) {
      setError(err.message);
    }
  };

  const fetchFacts = useCallback(async () => {
    if (!selectedSession) return;
    setLoading(true);
    try {
      const token = localStorage.getItem('velarix_token');
      const res = await fetch(`http://localhost:8080/v1/s/${selectedSession}/facts`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to fetch facts');
      const facts = await res.json();
      
      const transformedNodes = facts.map((f: any) => ({
        id: f.id,
        label: f.id.substring(0, 15),
        type: f.is_root ? 'root' : 'derived',
        valid: f.resolved_status >= 0.6,
        payload: f.payload,
        status: f.resolved_status
      }));

      const transformedEdges: any[] = [];
      facts.forEach((f: any) => {
        if (f.justification_sets) {
          f.justification_sets.forEach((set: string[]) => {
            set.forEach((parentId: string) => {
              transformedEdges.push({
                source: parentId,
                target: f.id,
                isValid: f.resolved_status >= 0.6 && transformedNodes.find((n:any) => n.id === parentId)?.valid
              });
            });
          });
        }
      });

      setGraphData({ nodes: transformedNodes, links: transformedEdges });
      
      // Update selected node if it exists
      if (selectedNode) {
         const updatedNode = transformedNodes.find((n:any) => n.id === selectedNode.id);
         setSelectedNode(updatedNode || null);
      }
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [selectedSession, selectedNode]);

  useEffect(() => {
    fetchSessions();
  }, []);

  useEffect(() => {
    if (selectedSession) {
      fetchFacts();
    }
  }, [selectedSession, fetchFacts]);

  const invalidateRoot = async (id: string) => {
    try {
      const token = localStorage.getItem('velarix_token');
      const res = await fetch(`http://localhost:8080/v1/s/${selectedSession}/facts/${id}/invalidate`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Invalidation failed');
      setTimeout(fetchFacts, 300);
    } catch (err: any) {
      alert(err.message);
    }
  };

  const resetState = async () => {
    try {
      const token = localStorage.getItem('velarix_token');
      await fetch(`http://localhost:8080/v1/s/${selectedSession}/revalidate`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      fetchFacts();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const drawNode = useCallback((node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
    const size = node.type === 'root' ? 8 : 5;
    const isValid = node.valid;
    const isSelected = selectedNode?.id === node.id;
    
    ctx.beginPath();
    ctx.arc(node.x, node.y, size, 0, 2 * Math.PI, false);
    ctx.fillStyle = isValid ? (node.type === 'root' ? '#06b6d4' : '#a855f7') : 'rgba(239, 68, 68, 0.3)';
    ctx.fill();
    
    if (isValid) {
       ctx.strokeStyle = node.type === 'root' ? '#06b6d4' : '#a855f7';
       ctx.lineWidth = 1;
       ctx.stroke();
       
       ctx.shadowColor = node.type === 'root' ? '#06b6d4' : '#a855f7';
       ctx.shadowBlur = isSelected ? 20 : 10;
    } else {
       ctx.strokeStyle = '#ef4444';
       ctx.lineWidth = 1;
       ctx.setLineDash([2, 2]);
       ctx.stroke();
       ctx.setLineDash([]);
    }
    
    if (isSelected) {
       ctx.beginPath();
       ctx.arc(node.x, node.y, size + 4, 0, 2 * Math.PI, false);
       ctx.strokeStyle = '#ffffff';
       ctx.lineWidth = 1;
       ctx.stroke();
    }
    
    ctx.shadowBlur = 0; // reset
    
    if (globalScale > 2) {
        ctx.font = `${3/globalScale + 2}px monospace`;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
        ctx.fillText(node.label, node.x, node.y + size + 4);
    }
 }, [selectedNode]);

  return (
    <div className="h-[calc(100vh-8rem)] flex flex-col space-y-4 animate-in fade-in duration-500 relative">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-2">
        <div>
          <h1 className="text-xl md:text-2xl font-display font-semibold text-white">Dominator Tree Visualizer</h1>
          <p className="text-xs md:text-sm text-v-text-muted mt-1">Physics-based causal state tracking and invalidation cascades.</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
           <div className="relative group z-20 flex-1 md:flex-none">
              <select 
                value={selectedSession} 
                onChange={(e) => setSelectedSession(e.target.value)}
                className="w-full md:w-auto appearance-none bg-v-bg-card border border-v-border text-white pl-4 pr-10 py-2 rounded-md text-sm focus:outline-none focus:border-v-accent transition-colors"
              >
                {sessions.length === 0 && <option>No active sessions</option>}
                {sessions.map(s => (
                  <option key={s.id} value={s.id}>{s.id}</option>
                ))}
              </select>
              <ChevronDown className="w-4 h-4 absolute right-3 top-1/2 -translate-y-1/2 text-v-text-muted pointer-events-none group-hover:text-white transition-colors" />
           </div>
           <button onClick={resetState} className="flex-1 md:flex-none flex items-center justify-center z-20 space-x-2 bg-v-bg-card border border-v-border text-white px-4 py-2 rounded-md font-medium text-sm hover:border-v-text-muted transition-colors shadow-sm">
              <RotateCcw className="w-4 h-4" />
              <span className="whitespace-nowrap">Revalidate</span>
           </button>
        </div>
      </div>

      <div className="flex-1 rounded-xl relative overflow-hidden bg-[#050505] border border-white/10 shadow-[inset_0_0_100px_rgba(0,0,0,0.8)] flex">
        
        {/* Graph Canvas */}
        <div className="flex-1 relative h-full">
            {loading && graphData.nodes.length === 0 && (
              <div className="absolute inset-0 z-50 flex flex-col items-center justify-center bg-black/60 backdrop-blur-sm">
                <Loader2 className="w-10 h-10 text-v-accent animate-spin mb-4" />
                <span className="text-v-text-muted font-mono text-sm">Syncing with Epistemic Engine...</span>
              </div>
            )}

            {error && (
              <div className="absolute inset-0 z-50 flex flex-col items-center justify-center bg-black/60 backdrop-blur-sm">
                <AlertCircle className="w-10 h-10 text-v-error mb-4" />
                <span className="text-white font-medium">{error}</span>
                <button onClick={fetchFacts} className="mt-4 text-v-accent hover:underline text-sm">Retry Connection</button>
              </div>
            )}

            <div className="absolute inset-0 pointer-events-none opacity-10" style={{ backgroundImage: 'radial-gradient(circle at 2px 2px, white 1px, transparent 0)', backgroundSize: '30px 30px' }}></div>
            
            <ForceGraph2D
                ref={graphRef}
                graphData={graphData}
                nodeCanvasObject={drawNode}
                nodeRelSize={6}
                linkColor={(link: any) => link.isValid ? 'rgba(63, 63, 70, 0.6)' : 'rgba(239, 68, 68, 0.4)'}
                linkWidth={1.5}
                linkDirectionalParticles={2}
                linkDirectionalParticleWidth={2}
                linkDirectionalParticleSpeed={0.01}
                linkDirectionalParticleColor={(link: any) => link.isValid ? '#06b6d4' : '#ef4444'}
                onNodeClick={(node) => setSelectedNode(node)}
                cooldownTicks={100}
                d3VelocityDecay={0.4}
            />

            {/* Legend Map */}
            <div className="absolute bottom-6 left-6 flex-wrap hidden sm:flex items-center gap-6 text-xs bg-black/50 p-3 rounded-lg border border-white/5 backdrop-blur-md z-10 pointer-events-none">
              <div className="flex items-center text-zinc-300">
                <div className="w-3 h-3 rounded-full bg-v-accent shadow-[0_0_10px_#06b6d4] mr-2"></div> Root
              </div>
              <div className="flex items-center text-zinc-300">
                <div className="w-3 h-3 rounded-full bg-purple-500 shadow-[0_0_10px_#a855f7] mr-2"></div> Derived
              </div>
              <div className="flex items-center text-zinc-300">
                <div className="w-3 h-3 rounded-full bg-transparent border border-dashed border-v-error mr-2"></div> Retracted
              </div>
            </div>
        </div>

        {/* Right Drawer */}
        <AnimatePresence>
           {selectedNode && (
             <motion.div 
               initial={{ x: '100%', opacity: 0 }}
               animate={{ x: 0, opacity: 1 }}
               exit={{ x: '100%', opacity: 0 }}
               transition={{ type: "spring", stiffness: 300, damping: 30 }}
               className="w-full sm:w-96 border-l border-white/10 bg-[#0a0a0a]/95 backdrop-blur-xl h-full absolute right-0 z-40 flex flex-col shadow-2xl"
             >
                <div className="p-6 border-b border-white/10 flex items-center justify-between">
                   <div className="flex items-center space-x-3">
                      <div className={`w-3 h-3 rounded-full ${selectedNode.valid ? (selectedNode.type === 'root' ? 'bg-v-accent' : 'bg-purple-500') : 'bg-v-error'} shadow-[0_0_10px_currentColor]`} />
                      <h2 className="text-white font-medium font-mono text-sm tracking-wider">NODE INSPECTOR</h2>
                   </div>
                   <button onClick={() => setSelectedNode(null)} className="text-v-text-muted hover:text-white transition-colors">
                     <X className="w-5 h-5" />
                   </button>
                </div>
                
                <div className="flex-1 overflow-y-auto p-6 space-y-6">
                   <div>
                      <div className="text-xs text-zinc-500 font-medium uppercase tracking-wider mb-2">Fact ID</div>
                      <div className="font-mono text-sm break-all text-v-accent bg-v-accent/10 px-3 py-2 rounded border border-v-accent/20">
                        {selectedNode.id}
                      </div>
                   </div>

                   <div>
                      <div className="text-xs text-zinc-500 font-medium uppercase tracking-wider mb-2">Status</div>
                      <div className="flex items-center space-x-2">
                         {selectedNode.valid ? (
                           <span className="px-2 py-1 bg-v-success/20 text-v-success text-xs font-bold uppercase rounded border border-v-success/30">Active</span>
                         ) : (
                           <span className="px-2 py-1 bg-v-error/20 text-v-error text-xs font-bold uppercase rounded border border-v-error/30">Retracted</span>
                         )}
                         <span className="text-sm font-mono text-zinc-400">Score: {selectedNode.status.toFixed(4)}</span>
                      </div>
                   </div>

                   <div>
                      <div className="text-xs text-zinc-500 font-medium uppercase tracking-wider mb-2">JSON Payload</div>
                      <pre className="bg-[#050505] p-4 rounded-lg border border-white/5 overflow-x-auto text-xs font-mono text-zinc-300 leading-relaxed">
                        {JSON.stringify(selectedNode.payload, null, 2)}
                      </pre>
                   </div>
                </div>

                {selectedNode.type === 'root' && selectedNode.valid && (
                  <div className="p-6 border-t border-white/10 bg-black/50">
                    <button 
                      onClick={() => invalidateRoot(selectedNode.id)}
                      className="w-full flex items-center justify-center space-x-2 bg-red-500/10 hover:bg-red-500/20 text-red-500 border border-red-500/30 py-3 rounded-lg transition-colors font-medium text-sm"
                    >
                      <ShieldAlert className="w-4 h-4" />
                      <span>Invalidate Root Fact</span>
                    </button>
                    <p className="text-xs text-zinc-600 mt-3 text-center leading-relaxed">This will trigger a causal cascade, pruning all derived facts.</p>
                  </div>
                )}
             </motion.div>
           )}
        </AnimatePresence>

      </div>
    </div>
  );
}
