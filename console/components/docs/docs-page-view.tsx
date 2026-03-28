"use client";

import { useEffect, useMemo, useState } from "react";

import { ChevronDown, ChevronRight, Menu, X } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { docsNav, type DocCodeBlock, type DocPage } from "./docs-data";

const mechanicalEase: [number, number, number, number] = [0.16, 1, 0.3, 1];
const siteNavItems = [
  { label: "Products", href: "/#products" },
  { label: "Solutions", href: "/#solutions" },
  { label: "Resources", href: "/#resources" },
  { label: "Pricing", href: "/pricing" },
  { label: "Documentation", href: "/docs" },
  { label: "Blog", href: "/#blog" },
];

function renderAccentText(text: string, accent?: string) {
  if (!accent) {
    return text;
  }

  const accentIndex = text.indexOf(accent);

  if (accentIndex === -1) {
    return text;
  }

  const before = text.slice(0, accentIndex);
  const after = text.slice(accentIndex + accent.length);

  return (
    <>
      {before}
      <span className="docs-serif font-medium italic text-white">{accent}</span>
      {after}
    </>
  );
}

function SiteLogo() {
  return (
    <div className="flex items-center gap-3">
      <div className="relative flex h-9 w-9 items-center justify-center rounded-full border border-white/12 bg-white/[0.04]">
        <div className="absolute h-4 w-4 rotate-45 border border-white/45" />
        <div className="h-1.5 w-1.5 rounded-full bg-white" />
      </div>
      <div>
        <p className="text-sm font-semibold tracking-[-0.03em] text-white">
          Velarix
        </p>
        <p className="font-mono text-[10px] uppercase tracking-[0.22em] text-zinc-500">
          Infrastructure
        </p>
      </div>
    </div>
  );
}

function SiteHeaderLink({
  label,
  href,
}: {
  label: string;
  href: string;
}) {
  const isCurrentDocs = href === "/docs";
  const showChevron = href.startsWith("/#");

  return (
    <Link
      href={href}
      className={`subtle-link inline-flex items-center gap-1.5 rounded-full px-3 py-2 text-sm tracking-[-0.01em] ${
        isCurrentDocs ? "text-white" : "text-zinc-400"
      }`}
    >
      <span>{label}</span>
      {showChevron ? (
        <ChevronDown className="h-3.5 w-3.5 text-zinc-600" />
      ) : null}
    </Link>
  );
}

function renderCodeToken(token: string, key: string) {
  if (!token) {
    return null;
  }

  if (token.startsWith("#") || token.startsWith("//")) {
    return (
      <span key={key} className="text-zinc-500">
        {token}
      </span>
    );
  }

  if (
    token.startsWith('"') ||
    token.startsWith("'") ||
    token.startsWith("`")
  ) {
    return (
      <span key={key} className="text-[#9ecbff]">
        {token}
      </span>
    );
  }

  if (
    /^(import|from|const|await|new|async|return|class|export|def|if|else|for|in|raise)$/.test(
      token,
    )
  ) {
    return (
      <span key={key} className="text-white">
        {token}
      </span>
    );
  }

  if (
    /^(OpenAI|VelarixClient|VelarixOpenAI|AsyncVelarixClient|session|client)$/.test(
      token,
    )
  ) {
    return (
      <span key={key} className="text-[#f6d68d]">
        {token}
      </span>
    );
  }

  if (/^(true|false|null|None)$/.test(token)) {
    return (
      <span key={key} className="text-[#c7b4ff]">
        {token}
      </span>
    );
  }

  return <span key={key}>{token}</span>;
}

function highlightCode(line: string) {
  if (!line.trim()) {
    return <span>&nbsp;</span>;
  }

  const tokens = line.split(
    /(#.*$|\/\/.*$|`(?:[^`\\]|\\.)*`|"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|\b(?:import|from|const|await|new|async|return|class|export|def|if|else|for|in|raise|OpenAI|VelarixClient|VelarixOpenAI|AsyncVelarixClient|session|client|true|false|null|None)\b)/g,
  );

  return tokens.map((token, index) =>
    renderCodeToken(token, `${line}-${index}`),
  );
}

function DocsCodeBlock({ block }: { block: DocCodeBlock }) {
  return (
    <div className="overflow-hidden rounded-[28px] border border-white/5 bg-[#09090b]">
      <div className="flex items-center justify-between border-b border-white/5 px-5 py-4">
        <div>
          <p className="text-sm font-semibold text-white">{block.title}</p>
          <p className="mt-1 text-[11px] uppercase tracking-[0.2em] text-zinc-600">
            {block.language}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="h-2.5 w-2.5 rounded-full bg-[#ff5f56]/80" />
          <span className="h-2.5 w-2.5 rounded-full bg-[#ffbd2e]/80" />
          <span className="h-2.5 w-2.5 rounded-full bg-[#27c93f]/80" />
        </div>
      </div>
      <div className="overflow-x-auto px-5 py-5">
        <pre className="min-w-full font-mono text-[13px] leading-7 text-zinc-300">
          {block.lines.map((line, index) => (
            <div key={`${block.title}-${index}`} className="whitespace-pre">
              {highlightCode(line)}
            </div>
          ))}
        </pre>
      </div>
    </div>
  );
}

function DocsCallout({
  tone,
  title,
  body,
}: {
  tone: "note" | "warning";
  title: string;
  body: string;
}) {
  const toneClass =
    tone === "note"
      ? "before:bg-logic-blue-400 before:shadow-[0_0_24px_rgba(74,156,255,0.22)]"
      : "before:bg-warning before:shadow-[0_0_24px_rgba(217,164,65,0.24)]";

  return (
    <div
      className={`relative overflow-hidden rounded-2xl border border-white/5 bg-white/[0.02] px-6 py-5 before:absolute before:bottom-5 before:left-0 before:top-5 before:w-px ${toneClass}`}
    >
      <p className="text-sm font-semibold text-white">{title}</p>
      <p className="mt-2 text-sm leading-7 text-zinc-400">{body}</p>
    </div>
  );
}

function DocsPanelGrid({
  panels,
}: {
  panels: NonNullable<DocPage["sections"][number]["panels"]>;
}) {
  return (
      <div className="grid gap-3 md:grid-cols-2">
      {panels.map((panel) => {
        const toneClass =
          panel.tone === "logic"
            ? "border-logic-blue-500/35 bg-logic-blue-900/20"
            : panel.tone === "warning"
              ? "border-warning/35 bg-warning/10"
              : panel.tone === "danger"
                ? "border-danger/35 bg-danger/10"
                : panel.tone === "success"
                  ? "border-success/35 bg-success/10"
                  : "border-border bg-surface/55";

        const labelClass =
          panel.tone === "logic"
            ? "text-logic-blue-200"
            : panel.tone === "warning"
              ? "text-warning"
              : panel.tone === "danger"
                ? "text-danger"
                : panel.tone === "success"
                  ? "text-success"
                  : "text-zinc-300";

        return (
          <div
            key={`${panel.title}-${panel.body}`}
            className={`rounded-md border p-3.5 ${toneClass}`}
          >
            <p className={`text-[13px] font-semibold ${labelClass}`}>{panel.title}</p>
            <p className="mt-1.5 text-[13px] leading-6 text-zinc-400">{panel.body}</p>
          </div>
        );
      })}
    </div>
  );
}

function BeliefGraphDiagram() {
  return (
    <div className="overflow-hidden rounded-[28px] border border-white/5 bg-black/[0.55] p-6">
      <div className="mb-4 flex items-center justify-between">
        <div>
          <p className="text-[13px] font-semibold text-white">Belief Graph</p>
          <p className="mt-1 text-[13px] text-zinc-500">
            Root assertions support derived operational state.
          </p>
        </div>
        <div className="rounded-full border border-white/10 bg-white/[0.03] px-3 py-1 text-[11px] uppercase tracking-[0.2em] text-zinc-500">
          Session View
        </div>
      </div>

      <svg
        viewBox="0 0 640 260"
        className="h-auto w-full"
        role="img"
        aria-label="Simplified belief graph diagram"
      >
        <defs>
          <linearGradient id="doc-line" x1="0%" x2="100%" y1="0%" y2="0%">
            <stop offset="0%" stopColor="rgba(255,255,255,0.14)" />
            <stop offset="100%" stopColor="rgba(255,255,255,0.04)" />
          </linearGradient>
        </defs>

        <path
          d="M170 72 L318 130"
          stroke="url(#doc-line)"
          strokeWidth="2"
          fill="none"
        />
        <path
          d="M170 192 L318 130"
          stroke="url(#doc-line)"
          strokeWidth="2"
          fill="none"
        />
        <path
          d="M368 130 L512 82"
          stroke="url(#doc-line)"
          strokeWidth="2"
          fill="none"
        />
        <path
          d="M368 130 L512 182"
          stroke="url(#doc-line)"
          strokeWidth="2"
          fill="none"
        />

        {[
          {
            x: 92,
            y: 44,
            label: "claim_submitted",
            tone: "rgba(74,156,255,0.16)",
          },
          {
            x: 92,
            y: 164,
            label: "policy_verified",
            tone: "rgba(74,156,255,0.16)",
          },
          {
            x: 260,
            y: 102,
            label: "eligible_for_review",
            tone: "rgba(255,255,255,0.06)",
          },
          {
            x: 454,
            y: 54,
            label: "manual_escalation",
            tone: "rgba(255,255,255,0.06)",
          },
          {
            x: 454,
            y: 154,
            label: "auto_hold",
            tone: "rgba(255,255,255,0.06)",
          },
        ].map((node) => (
          <g key={node.label}>
            <rect
              x={node.x}
              y={node.y}
              width="116"
              height="52"
              rx="22"
              fill={node.tone}
              stroke="rgba(255,255,255,0.08)"
            />
            <text
              x={node.x + 58}
              y={node.y + 30}
              fill="rgba(255,255,255,0.88)"
              fontSize="12"
              fontFamily="var(--font-doc-sans), var(--font-inter), sans-serif"
              textAnchor="middle"
            >
              {node.label}
            </text>
          </g>
        ))}
      </svg>
    </div>
  );
}

function SectionBlock({
  pageId,
  section,
}: {
  pageId: string;
  section: DocPage["sections"][number];
}) {
  const shouldReduceMotion = useReducedMotion();

  return (
    <motion.section
      id={section.id}
      initial={{ opacity: 0, y: shouldReduceMotion ? 0 : 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.18 }}
      transition={{ duration: 0.32, ease: mechanicalEase }}
      className="scroll-mt-24"
    >
      <div className="mb-4 flex items-baseline justify-between gap-4">
        <h2 className="text-[1.65rem] font-bold tracking-tight text-white">
          {section.title}
        </h2>
        <span className="font-mono text-[11px] uppercase tracking-[0.24em] text-zinc-600">
          {pageId}
        </span>
      </div>

      <div className="space-y-4 text-[14px] leading-relaxed text-zinc-400">
        {section.paragraphs?.map((paragraph) => (
          <p key={paragraph}>{paragraph}</p>
        ))}

        {section.bullets ? (
          <ul className="space-y-2.5">
            {section.bullets.map((bullet) => (
              <li key={bullet} className="flex gap-3">
                <span className="mt-2 h-1.5 w-1.5 rounded-full bg-zinc-500" />
                <span className="flex-1">{bullet}</span>
              </li>
            ))}
          </ul>
        ) : null}

        {section.panels ? <DocsPanelGrid panels={section.panels} /> : null}

        {section.diagram === "belief-graph" ? <BeliefGraphDiagram /> : null}

        {section.codeBlocks?.map((block) => (
          <DocsCodeBlock
            key={`${section.id}-${block.title}-${block.language}`}
            block={block}
          />
        ))}

        {section.callouts?.map((callout) => (
          <DocsCallout
            key={`${section.id}-${callout.title}`}
            tone={callout.tone}
            title={callout.title}
            body={callout.body}
          />
        ))}
      </div>
    </motion.section>
  );
}

function DocsSidebarNav({
  activeHref,
  expandedGroups,
  onToggleGroup,
  onNavigate,
}: {
  activeHref: string;
  expandedGroups: Record<string, boolean>;
  onToggleGroup: (groupId: string) => void;
  onNavigate?: () => void;
}) {
  return (
    <nav className="space-y-4">
      {docsNav.map((group) => {
        const isExpanded = expandedGroups[group.id];

        return (
          <div key={group.id}>
            <button
              type="button"
              onClick={() => onToggleGroup(group.id)}
              className="flex w-full items-center justify-between py-1.5 text-left text-[10px] font-medium uppercase tracking-[0.18em] text-zinc-600 transition-colors hover:text-zinc-400"
            >
              <span>{group.title}</span>
              {isExpanded ? (
                <ChevronDown className="h-4 w-4 text-zinc-600" />
              ) : (
                <ChevronRight className="h-4 w-4 text-zinc-600" />
              )}
            </button>

            <AnimatePresence initial={false}>
              {isExpanded ? (
                <motion.div
                  initial={{ height: 0, opacity: 0 }}
                  animate={{ height: "auto", opacity: 1 }}
                  exit={{ height: 0, opacity: 0 }}
                  transition={{ duration: 0.22, ease: mechanicalEase }}
                  className="overflow-hidden"
                >
                  <div className="space-y-1 pb-1 pt-0.5">
                    {group.items.map((item) => {
                      const isActive = item.href === activeHref;

                      return (
                        <Link
                          key={item.href}
                          href={item.href}
                          onClick={onNavigate}
                          className={`relative block overflow-hidden rounded-full py-2 pl-4.5 pr-3 text-[12px] font-medium transition-colors ${
                            isActive
                              ? "bg-white/5 text-white"
                              : "text-zinc-500 hover:text-zinc-300"
                          }`}
                        >
                          {isActive ? (
                            <motion.span
                              layoutId="docs-active-link-indicator"
                              className="absolute bottom-2 left-0 top-2 w-[2px] rounded-full bg-white"
                              transition={{ duration: 0.2, ease: mechanicalEase }}
                            />
                          ) : null}

                          <span
                            className={item.accent ? "docs-serif italic text-[14px]" : undefined}
                          >
                            {item.label}
                          </span>
                        </Link>
                      );
                    })}
                  </div>
                </motion.div>
              ) : null}
            </AnimatePresence>
          </div>
        );
      })}
    </nav>
  );
}

export function DocsPageView({ page }: { page: DocPage }) {
  const pathname = usePathname();
  const shouldReduceMotion = useReducedMotion();
  const activeHref = pathname || `/docs/${page.slug.join("/")}`;
  const tocItems = useMemo(
    () => page.sections.map((section) => ({ id: section.id, label: section.title })),
    [page.sections],
  );

  const activeGroupId =
    docsNav.find((group) => group.items.some((item) => item.href === activeHref))
      ?.id ?? page.groupId;

  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(
      docsNav.map((group, index) => [
        group.id,
        group.id === activeGroupId || (index === 0 && !activeGroupId),
      ]),
    ),
  );
  const [activeSection, setActiveSection] = useState(tocItems[0]?.id ?? "");

  useEffect(() => {
    if (!tocItems.length) {
      return undefined;
    }

    const sections = tocItems
      .map((item) => document.getElementById(item.id))
      .filter((section): section is HTMLElement => Boolean(section));

    if (!sections.length) {
      return undefined;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const visibleEntries = entries
          .filter((entry) => entry.isIntersecting)
          .sort(
            (left, right) =>
              Math.abs(left.boundingClientRect.top) -
              Math.abs(right.boundingClientRect.top),
          );

        if (visibleEntries[0]) {
          setActiveSection(visibleEntries[0].target.id);
        }
      },
      {
        rootMargin: "-18% 0px -62% 0px",
        threshold: [0.15, 0.45, 0.7],
      },
    );

    sections.forEach((section) => observer.observe(section));

    return () => observer.disconnect();
  }, [tocItems]);

  function toggleGroup(groupId: string) {
    setExpandedGroups((current) => ({
      ...current,
      [groupId]: !current[groupId],
    }));
  }

  return (
    <div className="min-h-screen bg-[#0A0A0C] text-white">
      <header className="sticky top-0 z-50 border-b border-white/5 bg-[#0A0A0C]/80 backdrop-blur-md">
        <div className="flex min-h-16 w-full items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
          <div className="flex items-center gap-2 sm:gap-3">
            <div className="md:hidden">
              <button
                type="button"
                onClick={() => setIsDrawerOpen(true)}
                className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-white/10 bg-white/[0.03] text-zinc-300 transition-colors hover:text-white md:hidden"
                aria-label="Open documentation navigation"
              >
                <Menu className="h-4 w-4" />
              </button>
            </div>

            <Link href="/" className="rounded-full px-1 py-1">
              <SiteLogo />
            </Link>
          </div>

          <nav className="hidden items-center gap-1 lg:flex">
            {siteNavItems.map((item) => (
              <SiteHeaderLink
                key={item.label}
                label={item.label}
                href={item.href}
              />
            ))}
          </nav>

          <div className="flex items-center gap-2">
            <Link
              href="/docs"
              className="shimmer-button inline-flex items-center justify-center rounded-full bg-white px-4 py-2 text-sm font-semibold text-black"
            >
              Dashboard
            </Link>
            <div className="flex h-9 w-9 items-center justify-center rounded-full border border-white/12 bg-white/[0.04] font-mono text-[10px] uppercase tracking-[0.18em] text-zinc-300">
              VX
            </div>
          </div>
        </div>
      </header>

      <AnimatePresence>
        {isDrawerOpen ? (
          <>
            <motion.button
              type="button"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.2, ease: mechanicalEase }}
              className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm md:hidden"
              onClick={() => setIsDrawerOpen(false)}
              aria-label="Close documentation navigation"
            />
            <motion.aside
              initial={{ x: shouldReduceMotion ? 0 : -28, opacity: 0 }}
              animate={{ x: 0, opacity: 1 }}
              exit={{ x: shouldReduceMotion ? 0 : -20, opacity: 0 }}
              transition={{ duration: 0.22, ease: mechanicalEase }}
              className="fixed inset-y-0 left-0 z-50 w-64 border-r border-white/5 bg-[#0A0A0C] px-4 pb-6 pt-6 md:hidden"
            >
              <div className="mb-6 flex items-center justify-between">
                <p className="text-sm font-semibold text-white">Documentation</p>
                <button
                  type="button"
                  onClick={() => setIsDrawerOpen(false)}
                  className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-white/10 bg-white/[0.03] text-zinc-300 transition-colors hover:text-white"
                  aria-label="Close documentation navigation"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              <div className="overflow-y-auto pb-8">
                <DocsSidebarNav
                  activeHref={activeHref}
                  expandedGroups={expandedGroups}
                  onToggleGroup={toggleGroup}
                  onNavigate={() => setIsDrawerOpen(false)}
                />
              </div>
            </motion.aside>
          </>
        ) : null}
      </AnimatePresence>

      <div className="flex w-full">
        <aside className="hidden w-60 shrink-0 md:block">
          <div className="sticky top-16 flex h-[calc(100vh-4rem)] flex-col border-r border-white/5 bg-[#0A0A0C] px-4 pb-6 pt-6">
            <div className="mb-5">
              <p className="text-[11px] font-medium uppercase tracking-[0.24em] text-zinc-600">
                Documentation
              </p>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto">
              <DocsSidebarNav
                activeHref={activeHref}
                expandedGroups={expandedGroups}
                onToggleGroup={toggleGroup}
              />
            </div>
          </div>
        </aside>

        <main className="min-w-0 flex-1 px-4 pb-20 pt-8 sm:px-5 md:px-7 lg:px-10">
          <article className="mx-auto max-w-3xl">
            <motion.div
              initial={{ opacity: 0, y: shouldReduceMotion ? 0 : 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, ease: mechanicalEase }}
              className="border-b border-white/5 pb-8"
            >
              <p className="text-[11px] font-medium uppercase tracking-[0.24em] text-zinc-500">
                {page.eyebrow}
              </p>
              <h1 className="mt-3 text-3xl font-bold tracking-tight text-white sm:text-4xl">
                {renderAccentText(page.title, page.titleAccent)}
              </h1>
              <p className="mt-4 max-w-2xl text-[14px] leading-relaxed text-zinc-400">
                {page.description}
              </p>
            </motion.div>

            <div className="space-y-12 py-10">
              {page.sections.map((section) => (
                <SectionBlock
                  key={section.id}
                  pageId={`${page.slug[0]}.${section.id}`}
                  section={section}
                />
              ))}
            </div>
          </article>
        </main>

        <aside className="hidden w-52 shrink-0 xl:block">
          <div className="sticky top-20 pr-5">
            <p className="text-[10px] font-medium uppercase tracking-[0.22em] text-zinc-600">
              On this page
            </p>
            <div className="mt-3 space-y-1">
              {tocItems.map((item) => {
                const isActive = item.id === activeSection;

                return (
                  <Link
                    key={item.id}
                    href={`#${item.id}`}
                    onClick={() => setActiveSection(item.id)}
                    className={`relative block overflow-hidden rounded-full px-3 py-1.5 text-[12px] font-medium transition-colors ${
                      isActive
                        ? "text-white"
                        : "text-zinc-500 hover:text-zinc-300"
                    }`}
                  >
                    {isActive ? (
                      <motion.span
                        layoutId="docs-toc-highlight"
                        className="absolute inset-0 rounded-full bg-white/5"
                        transition={{ duration: 0.22, ease: mechanicalEase }}
                      />
                    ) : null}
                    <span className="relative">{item.label}</span>
                  </Link>
                );
              })}
            </div>
          </div>
        </aside>
      </div>
    </div>
  );
}
