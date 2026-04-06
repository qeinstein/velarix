"use client";

import { useEffect, useState } from "react";

const frames = [
  {
    badge: "Standard client",
    summary: "Stateless request flow",
    code: `from openai import OpenAI

client = OpenAI()

response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Execute..."}],
)`,
  },
  {
    badge: "With Velarix",
    summary: "Causal memory attached",
    code: `from velarix.adapters.openai import OpenAI

client = OpenAI(velarix_session_id="research-1")

response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Execute..."}],
)`,
  },
];

export default function CodeSwapPanel() {
  const [activeFrame, setActiveFrame] = useState(0);

  useEffect(() => {
    const interval = window.setInterval(() => {
      setActiveFrame((current) => (current + 1) % frames.length);
    }, 4800);

    return () => window.clearInterval(interval);
  }, []);

  return (
    <div className="dark-surface overflow-hidden rounded-xl border border-white/10 bg-[var(--code-bg)]">
      <div className="flex items-center justify-between border-b border-white/10 bg-white/5 px-4 py-3">
        <div className="flex gap-2">
          <div className="h-2.5 w-2.5 rounded-full bg-[#ff5f56]" />
          <div className="h-2.5 w-2.5 rounded-full bg-[#ffbd2e]" />
          <div className="h-2.5 w-2.5 rounded-full bg-[#27c93f]" />
        </div>
        <span className="font-mono text-[0.65rem] uppercase tracking-[0.16em] text-white/30">
          runtime.py
        </span>
      </div>

      <div className="relative min-height-[14rem] p-5 md:p-7">
        {frames.map((frame, index) => (
          <div
            key={frame.badge}
            className={`transition-all duration-700 ease-in-out ${
              activeFrame === index 
                ? "relative opacity-100 translate-y-0" 
                : "absolute inset-x-5 inset-y-5 opacity-0 translate-y-4 pointer-events-none md:inset-x-7 md:inset-y-7"
            }`}
            aria-hidden={activeFrame !== index}
          >
            <div className="mb-4 flex items-center justify-between gap-4">
              <span className="flex items-center gap-2 font-mono text-[0.68rem] uppercase tracking-[0.16em] text-white/70">
                <span className={`h-1.5 w-1.5 rounded-full ${index === 1 ? 'bg-[#27c93f]' : 'bg-white/20'}`} />
                {frame.badge}
              </span>
              <span className="font-mono text-[0.65rem] uppercase tracking-[0.16em] text-white/30">
                {frame.summary}
              </span>
            </div>
            <pre className="font-mono text-[0.88rem] leading-7 text-white/90 overflow-x-auto">
              {frame.code}
            </pre>
          </div>
        ))}
      </div>
    </div>
  );
}
