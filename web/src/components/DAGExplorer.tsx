"use client";

import { useEffect, useRef, useState } from "react";
import ForceGraph3D from "react-force-graph-3d";

type GraphNode = {
  id: string;
  is_root?: boolean;
  status?: number;
  x?: number;
  y?: number;
  z?: number;
};

type GraphData = {
  nodes: GraphNode[];
  links: Array<{
    source: string | GraphNode;
    target: string | GraphNode;
  }>;
};

export default function DAGExplorer({
  graphData,
  onNodeClick,
}: {
  graphData: GraphData;
  onNodeClick: (node: GraphNode) => void;
}) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const fgRef = useRef<any>(null);
  const [dimensions, setDimensions] = useState({ height: 500, width: 800 });

  useEffect(() => {
    if (!containerRef.current) {
      return;
    }

    const updateDimensions = () => {
      if (!containerRef.current) {
        return;
      }

      setDimensions({
        height: containerRef.current.clientHeight,
        width: containerRef.current.clientWidth,
      });
    };

    updateDimensions();

    const observer = new ResizeObserver(updateDimensions);
    observer.observe(containerRef.current);

    return () => observer.disconnect();
  }, []);

  const handleNodeClick = (node: GraphNode) => {
    const distance = 100;
    const x = node.x ?? 0;
    const y = node.y ?? 0;
    const z = node.z ?? 0;
    const distRatio = 1 + distance / Math.max(Math.hypot(x, y, z), 1);

    if (fgRef.current) {
      fgRef.current.cameraPosition({ x: x * distRatio, y: y * distRatio, z: z * distRatio }, node, 3000);
    }

    onNodeClick(node);
  };

  return (
    <div ref={containerRef} className="h-full w-full bg-black cursor-crosshair">
      {dimensions.width > 0 && dimensions.height > 0 ? (
        <ForceGraph3D
          ref={fgRef}
          graphData={graphData}
          nodeColor={(node: GraphNode) => {
            if (node.is_root) {
              return "#ffffff";
            }

            return (node.status ?? 0) >= 0.6 ? "#aaaaaa" : "#333333";
          }}
          nodeLabel={(node: GraphNode) => `Fact: ${node.id}\nStatus: ${node.status ?? 0}`}
          linkColor={() => "#333333"}
          backgroundColor="#000000"
          onNodeClick={handleNodeClick}
          nodeRelSize={6}
          linkWidth={1}
          linkDirectionalParticles={2}
          linkDirectionalParticleSpeed={0.005}
          linkDirectionalParticleColor={() => "#555555"}
          enableNodeDrag={false}
          width={dimensions.width}
          height={dimensions.height}
        />
      ) : null}
    </div>
  );
}
