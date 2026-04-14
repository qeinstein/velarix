import { ReactNode } from "react";

const variants = {
  note: {
    border: "border-[var(--line)]",
    bg: "bg-[var(--panel)]",
    label: "Note",
    labelColor: "text-[var(--muted)]",
  },
  warning: {
    border: "border-yellow-500/40",
    bg: "bg-yellow-500/5",
    label: "Warning",
    labelColor: "text-yellow-500",
  },
  danger: {
    border: "border-red-500/40",
    bg: "bg-red-500/5",
    label: "Danger",
    labelColor: "text-red-500",
  },
  tip: {
    border: "border-emerald-500/40",
    bg: "bg-emerald-500/5",
    label: "Tip",
    labelColor: "text-emerald-500",
  },
} as const;

export function Callout({
  type = "note",
  children,
}: {
  type?: keyof typeof variants;
  children: ReactNode;
}) {
  const v = variants[type];
  return (
    <div className={`my-6 rounded-xl border px-5 py-4 ${v.border} ${v.bg}`}>
      <p className={`font-mono text-[0.7rem] uppercase tracking-[0.18em] mb-2 ${v.labelColor}`}>
        {v.label}
      </p>
      <div className="font-copy text-base leading-7 [&>p]:mb-0">{children}</div>
    </div>
  );
}
