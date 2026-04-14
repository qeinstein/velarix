"use client";

import { DocMetadata } from "@/lib/docs";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, useEffect } from "react";

function Sidebar({ docs }: { docs: DocMetadata[] }) {
  const pathname = usePathname();

  const sections = docs.reduce(
    (groups, doc) => {
      if (!groups[doc.section]) groups[doc.section] = [];
      groups[doc.section].push(doc);
      return groups;
    },
    {} as Record<string, DocMetadata[]>,
  );

  return (
    <aside className="docs-sidebar">
      <div className="docs-sidebar-inner">
        <div className="docs-sidebar-brand">
          <Link href="/" className="docs-brand-link">
            Velarix
          </Link>
          <span className="docs-brand-badge">Docs</span>
        </div>

        <nav className="docs-nav">
          {Object.entries(sections).map(([section, items]) => (
            <div key={section} className="docs-nav-section">
              <p className="docs-nav-label">{section}</p>
              {items.map((doc) => {
                const active = pathname === `/docs/${doc.slug}`;
                return (
                  <Link
                    key={doc.slug}
                    href={`/docs/${doc.slug}`}
                    className={`docs-nav-item${active ? " is-active" : ""}`}
                  >
                    {doc.title}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>

        <div className="docs-sidebar-footer">
          <Link href="/" className="docs-footer-link">
            ← Back to site
          </Link>
          <a
            href="https://github.com/qeinstein/velarix"
            target="_blank"
            rel="noreferrer"
            className="docs-footer-link"
          >
            GitHub ↗
          </a>
        </div>
      </div>
    </aside>
  );
}

export function DocsShell({
  docs,
  children,
}: {
  docs: DocMetadata[];
  children: React.ReactNode;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const pathname = usePathname();

  useEffect(() => {
    setMenuOpen(false);
  }, [pathname]);

  return (
    <div className="docs-root">
      {/* Mobile header */}
      <header className="docs-mobile-header">
        <Link href="/" className="docs-brand-link">
          Velarix <span className="docs-brand-badge">Docs</span>
        </Link>
        <button
          className="docs-mobile-menu-btn"
          onClick={() => setMenuOpen((o) => !o)}
          aria-label="Toggle menu"
        >
          {menuOpen ? "✕" : "☰"}
        </button>
      </header>

      <div className={`docs-sidebar-wrap${menuOpen ? " is-open" : ""}`}>
        <Sidebar docs={docs} />
      </div>

      {menuOpen && (
        <div className="docs-overlay" onClick={() => setMenuOpen(false)} />
      )}

      <main className="docs-main">
        <div className="docs-content">{children}</div>
      </main>
    </div>
  );
}
