import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Background,
  Controls,
  Handle,
  MarkerType,
  MiniMap,
  Position,
  ReactFlow,
  BaseEdge,
  getBezierPath,
} from '@xyflow/react';
import type { EdgeProps, Edge, Node, NodeProps } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import dagre from 'dagre';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { motion, AnimatePresence } from 'framer-motion';
import type { Fact } from './lib/types';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

type FactNodeData = Record<string, unknown> & {
  label: string;
  status: number;
  isRoot: boolean;
  supportCount: number;
  hasViolations: boolean;
  isImpacted: boolean;
  isProvenance: boolean;
  isWhatIf: boolean;
  wasJustCollapsed: boolean;
};

type FactFlowNode = Node<FactNodeData, 'fact'>;

function FactNode({ data, selected }: NodeProps<FactFlowNode>) {
  const isValid = data.status === 1;
  const isImpacted = data.isImpacted;
  const isProvenance = data.isProvenance;
  const isWhatIf = data.isWhatIf;
  const wasJustCollapsed = data.wasJustCollapsed;
  const hasViolations = data.hasViolations;

  return (
    <motion.div
      animate={{ y: [0, -3, 0] }}
      transition={{ duration: 4, repeat: Infinity, ease: "easeInOut", delay: Math.random() * 2 }}
      className={cn(
        'relative min-w-[230px] overflow-hidden rounded-[1.4rem] border px-4 py-4 text-left shadow-[0_20px_48px_rgba(2,6,23,0.48)] transition-all duration-500',
        'bg-[#060b16]',
        isValid ? 'border-emerald-400/20' : 'border-rose-400/18',
        hasViolations && 'border-amber-300/22',
        selected && 'scale-[1.02] border-sky-400/50 shadow-[0_0_30px_rgba(56,189,248,0.2)]',
        isImpacted && 'border-rose-400/40 shadow-[0_0_30px_rgba(251,113,133,0.2)]',
        isProvenance && 'border-sky-400/40 shadow-[0_0_30px_rgba(56,189,248,0.2)]',
        isWhatIf && 'border-amber-400/40 shadow-[0_0_30px_rgba(251,191,36,0.2)]',
      )}
    >
      <div className={cn('absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/20 to-transparent')} />
      
      {/* Pulse effect */}
      <motion.div
        animate={{ opacity: [0.1, 0.2, 0.1], scale: [1, 1.2, 1] }}
        transition={{ duration: 3, repeat: Infinity, ease: "easeInOut" }}
        className={cn(
          'absolute -right-4 -top-4 h-24 w-24 rounded-full blur-2xl transition-opacity',
          isValid ? 'bg-emerald-400/20' : 'bg-rose-400/20',
          hasViolations && 'bg-amber-400/20',
        )}
      />

      {/* Ripple Effect for Invalidation Cascade */}
      <AnimatePresence>
        {wasJustCollapsed && (
          <motion.div 
            initial={{ scale: 0.8, opacity: 1 }}
            animate={{ scale: 2.5, opacity: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 1, ease: "easeOut" }}
            className={cn(
              'absolute inset-0 rounded-full border-4 pointer-events-none -z-10',
              hasViolations ? 'border-amber-400' : 'border-rose-500'
            )}
          />
        )}
      </AnimatePresence>

      <div className="relative z-10">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="text-[0.62rem] uppercase tracking-[0.18em] text-slate-500">
              {data.isRoot ? 'Root premise' : 'Derived belief'}
            </div>
            <div className="mt-2 text-sm font-bold text-white">{data.label}</div>
          </div>
          <div className={cn('rounded-full px-2.5 py-1 text-[0.6rem] uppercase font-bold tracking-[0.16em] shadow-inner', isValid ? 'bg-emerald-500/10 text-emerald-300 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-300 border border-rose-500/20')}>
            {isValid ? 'Valid' : 'Collapsed'}
          </div>
        </div>

        <div className="mt-4 flex flex-wrap gap-2 text-[0.62rem] uppercase tracking-[0.16em] font-semibold text-slate-400">
          <span className="rounded-full border border-white/10 bg-white/[0.03] px-2.5 py-1">Support {data.supportCount}</span>
          {hasViolations && <span className="rounded-full border border-amber-400/20 bg-amber-500/10 px-2.5 py-1 text-amber-300">Schema warn</span>}
          {isImpacted && <span className="rounded-full border border-rose-400/20 bg-rose-500/10 px-2.5 py-1 text-rose-300">Impact</span>}
          {isProvenance && <span className="rounded-full border border-sky-400/20 bg-sky-500/10 px-2.5 py-1 text-sky-300">Provenance</span>}
          {isWhatIf && <span className="rounded-full border border-amber-400/20 bg-amber-500/10 px-2.5 py-1 text-amber-300">What-if</span>}
        </div>
      </div>

      <Handle type="target" position={Position.Top} className="!h-2 !w-2 !border-0 !bg-sky-400" />
      <Handle type="source" position={Position.Bottom} className="!h-2 !w-2 !border-0 !bg-sky-400" />
    </motion.div>
  );
}

function FlowingEdge({
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  style = {},
  markerEnd,
}: EdgeProps) {
  const [edgePath] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  return (
    <>
      <BaseEdge path={edgePath} markerEnd={markerEnd} style={style} />
      <circle r="4" fill={style.stroke as string || "#38bdf8"} style={{ filter: 'drop-shadow(0px 0px 4px rgba(56,189,248,0.8))' }}>
        <animateMotion
          dur="2s"
          repeatCount="indefinite"
          path={edgePath}
        />
      </circle>
    </>
  );
}

const nodeTypes = { fact: FactNode };
const edgeTypes = { flowing: FlowingEdge };

function getLayoutedElements(nodes: FactFlowNode[], edges: Edge[]) {
  const graph = new dagre.graphlib.Graph();
  graph.setDefaultEdgeLabel(() => ({}));
  graph.setGraph({ rankdir: 'TB', ranksep: 120, nodesep: 90 });

  nodes.forEach((node) => graph.setNode(node.id, { width: 250, height: 120 }));
  edges.forEach((edge) => graph.setEdge(edge.source, edge.target));
  dagre.layout(graph);

  return {
    nodes: nodes.map((node) => {
      const layout = graph.node(node.id);
      return {
        ...node,
        position: {
          x: layout.x - 125,
          y: layout.y - 60,
        },
      };
    }),
    edges,
  };
}

interface GraphProps {
  facts: Record<string, Fact>;
  onNodeClick: (factId: string) => void;
  impactedNodeIds?: Set<string>;
  provenanceNodeIds?: Set<string>;
  whatIfNodeIds?: Set<string>;
}

export function Graph({ facts, onNodeClick, impactedNodeIds, provenanceNodeIds, whatIfNodeIds }: GraphProps) {
  const prevFacts = useRef<Record<string, Fact>>({});
  const [justCollapsed, setJustCollapsed] = useState<Set<string>>(new Set());

  useEffect(() => {
    const collapsed = new Set<string>();

    Object.keys(facts).forEach((id) => {
      const previous = prevFacts.current[id];
      const current = facts[id];
      if (previous && previous.resolved_status === 1 && (current.resolved_status ?? 0) < 1) {
        collapsed.add(id);
      }
    });

    prevFacts.current = facts;

    if (collapsed.size === 0) return undefined;

    const showTimer = window.setTimeout(() => setJustCollapsed(collapsed), 0);
    const clearTimer = window.setTimeout(() => setJustCollapsed(new Set()), 1800);

    return () => {
      window.clearTimeout(showTimer);
      window.clearTimeout(clearTimer);
    };
  }, [facts]);

  const { nodes, edges } = useMemo(() => {
    const initialNodes: FactFlowNode[] = [];
    const initialEdges: Edge[] = [];

    Object.values(facts).forEach((fact) => {
      initialNodes.push({
        id: fact.ID,
        type: 'fact',
        position: { x: 0, y: 0 },
        data: {
          label: fact.ID,
          status: fact.resolved_status ?? 0,
          isRoot: fact.IsRoot,
          supportCount: fact.ValidJustificationCount,
          hasViolations: Boolean(fact.validation_errors?.length),
          isImpacted: Boolean(impactedNodeIds?.has(fact.ID)),
          isProvenance: Boolean(provenanceNodeIds?.has(fact.ID)),
          wasJustCollapsed: justCollapsed.has(fact.ID),
          isWhatIf: Boolean(whatIfNodeIds?.has(fact.ID)),
        },
      });

      fact.justification_sets?.forEach((set, setIndex) => {
        set.forEach((parentId) => {
          const isImpacted = Boolean(impactedNodeIds?.has(fact.ID) && impactedNodeIds?.has(parentId));
          const isProvenance = Boolean(provenanceNodeIds?.has(fact.ID) && provenanceNodeIds?.has(parentId));
          const isWhatIf = Boolean(whatIfNodeIds?.has(fact.ID) && whatIfNodeIds?.has(parentId));
          const accent = isProvenance ? '#38bdf8' : isImpacted ? '#fb7185' : isWhatIf ? '#fbbf24' : '#475569';

          initialEdges.push({
            id: `edge-${parentId}-${fact.ID}-${setIndex}`,
            source: parentId,
            target: fact.ID,
            type: 'flowing', // Use custom flowing particle edge
            style: {
              stroke: accent,
              strokeWidth: isImpacted || isProvenance || isWhatIf ? 2.5 : 1.4,
              opacity: isImpacted || isProvenance || isWhatIf ? 1 : 0.46,
            },
            markerEnd: {
              type: MarkerType.ArrowClosed,
              color: accent,
            },
          });
        });
      });
    });

    return getLayoutedElements(initialNodes, initialEdges);
  }, [facts, impactedNodeIds, provenanceNodeIds, whatIfNodeIds, justCollapsed]);

  if (nodes.length === 0) {
    return (
      <div className="graph-canvas flex h-full items-center justify-center">
        <motion.div 
          initial={{ opacity: 0, scale: 0.95 }}
          animate={{ opacity: 1, scale: 1 }}
          className="surface-panel rounded-[2rem] px-8 py-10 text-center"
        >
          <div className="font-display text-2xl font-semibold text-white">No graph data yet</div>
          <p className="mt-3 max-w-md text-sm leading-7 text-slate-400">
            Authenticate, select a session, and load facts before expecting the graph view to prove anything.
          </p>
        </motion.div>
      </div>
    );
  }

  return (
    <div className="graph-canvas h-full w-full">
      <ReactFlow<FactFlowNode, Edge>
        fitView
        minZoom={0.2}
        maxZoom={1.75}
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitViewOptions={{ padding: 0.18 }}
        onNodeClick={(_, node) => onNodeClick(node.id)}
      >
        <Background gap={32} size={1} />
        <MiniMap
          pannable
          zoomable
          nodeColor={(node) => {
            const data = node.data as unknown as FactNodeData | undefined;
            if (data?.isImpacted) return '#fb7185';
            if (data?.isProvenance) return '#38bdf8';
            return data?.status === 1 ? '#34d399' : '#fda4af';
          }}
          maskColor="rgba(3, 7, 18, 0.82)"
          style={{ background: 'rgba(5, 10, 20, 0.82)', border: '1px solid rgba(148, 163, 184, 0.14)' }}
        />
        <Controls showInteractive={false} />
      </ReactFlow>
    </div>
  );
}