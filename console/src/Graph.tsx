import { useMemo } from 'react';
import { 
  ReactFlow, 
  Controls, 
  Background, 
  MarkerType,
  Handle,
  Position
} from '@xyflow/react';
import type { Node, Edge } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import dagre from 'dagre';
import type { Fact } from './lib/types';
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// Custom Neural Node Component
const FactNode = ({ data, selected }: any) => {
  const isValid = data.status === 1;
  const isViolated = data.hasViolations;
  const isImpacted = data.isImpacted;
  const isProvenance = data.isProvenance;

  return (
    <div className={cn(
      "relative px-4 py-3 rounded-xl border-2 transition-all duration-500 min-w-[180px]",
      "bg-slate-900 shadow-2xl",
      isValid ? "border-emerald-500/30" : "border-red-500/30",
      isViolated && "border-violet-500/50 shadow-violet-500/20",
      selected && "border-indigo-500 ring-4 ring-indigo-500/20 scale-105 z-50",
      isImpacted && "ring-4 ring-red-500/40 animate-pulse scale-105 z-50",
      isProvenance && "ring-4 ring-blue-500/40 scale-105 z-50"
    )}>
      {/* Neural Glow Effect */}
      <div className={cn(
        "absolute inset-0 rounded-xl opacity-20 blur-xl transition-opacity",
        isValid ? "bg-emerald-500" : "bg-red-500",
        isViolated && "bg-violet-500 opacity-40"
      )} />

      <div className="relative z-10">
        <div className="flex items-center justify-between gap-2 mb-1">
          <div className="text-[10px] font-bold text-slate-500 uppercase tracking-tighter truncate max-w-[120px]">
            {data.label}
          </div>
          <div className={cn(
            "w-2 h-2 rounded-full shadow-[0_0_8px_rgba(0,0,0,0.5)]",
            isValid ? "bg-emerald-400 shadow-emerald-400/50" : "bg-red-400 shadow-red-400/50",
            isViolated && "bg-violet-400 shadow-violet-400/50"
          )} />
        </div>
        
        <div className="text-[9px] font-medium text-slate-400 leading-tight line-clamp-1">
          {data.isRoot ? "ROOT PREMISE" : "DERIVED BELIEF"}
        </div>
      </div>

      <Handle type="target" position={Position.Top} className="opacity-0" />
      <Handle type="source" position={Position.Bottom} className="opacity-0" />
    </div>
  );
};

const nodeTypes = {
  fact: FactNode,
};

// Dagre layouting for the architecture feel
const getLayoutedElements = (nodes: Node[], edges: Edge[]) => {
  const dagreGraph = new dagre.graphlib.Graph();
  dagreGraph.setDefaultEdgeLabel(() => ({}));
  dagreGraph.setGraph({ rankdir: 'TB', ranksep: 100, nodesep: 80 });

  nodes.forEach((node) => {
    dagreGraph.setNode(node.id, { width: 220, height: 80 });
  });

  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });

  dagre.layout(dagreGraph);

  const layoutedNodes = nodes.map((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    return {
      ...node,
      position: {
        x: nodeWithPosition.x - 110,
        y: nodeWithPosition.y - 40,
      },
    };
  });

  return { nodes: layoutedNodes, edges };
};

interface GraphProps {
  facts: Record<string, Fact>;
  onNodeClick: (factId: string) => void;
  impactedNodeIds?: Set<string>;
  provenanceNodeIds?: Set<string>;
}

export function Graph({ facts, onNodeClick, impactedNodeIds, provenanceNodeIds }: GraphProps) {
  const { nodes, edges } = useMemo(() => {
    const initialNodes: Node[] = [];
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
          hasViolations: fact.validation_errors && fact.validation_errors.length > 0,
          isImpacted: impactedNodeIds?.has(fact.ID),
          isProvenance: provenanceNodeIds?.has(fact.ID)
        },
      });

      if (fact.justification_sets) {
        fact.justification_sets.forEach((set, setIndex) => {
          set.forEach((parentId) => {
            const isProvenance = provenanceNodeIds?.has(fact.ID) && provenanceNodeIds?.has(parentId);
            const isImpacted = impactedNodeIds?.has(fact.ID) && impactedNodeIds?.has(parentId);

            initialEdges.push({
              id: `e-${parentId}-${fact.ID}-${setIndex}`,
              source: parentId,
              target: fact.ID,
              animated: isProvenance || isImpacted,
              markerEnd: {
                type: MarkerType.ArrowClosed,
                width: 15,
                height: 15,
                color: isProvenance ? '#3b82f6' : isImpacted ? '#ef4444' : '#334155',
              },
              style: {
                strokeWidth: isProvenance || isImpacted ? 3 : 1.5,
                stroke: isProvenance ? '#3b82f6' : isImpacted ? '#ef4444' : '#334155',
                opacity: isProvenance || isImpacted ? 1 : 0.4
              },
            });
          });
        });
      }
    });

    return getLayoutedElements(initialNodes, initialEdges);
  }, [facts, impactedNodeIds, provenanceNodeIds]);

  return (
    <div className="w-full h-full bg-[#020617]">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodeClick={(_, node) => onNodeClick(node.id)}
        fitView
        minZoom={0.05}
        maxZoom={1.5}
      >
        <Background color="#1e293b" gap={24} size={1} />
        <Controls className="bg-slate-900 border-slate-800 fill-white" />
      </ReactFlow>
    </div>
  );
}
