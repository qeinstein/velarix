import { getDocsList } from "@/lib/docs";
import Link from "next/link";

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const docs = getDocsList();
  const sections = docs.reduce((groups, doc) => {
    if (!groups[doc.section]) {
      groups[doc.section] = [];
    }
    groups[doc.section].push(doc);
    return groups;
  }, {} as Record<string, typeof docs>);

  return (
    <div className="flex flex-col gap-8 pb-24 pt-8 md:flex-row">
      <aside className="w-full flex-shrink-0 md:w-72">
        <div className="sticky top-24 space-y-6">
          <div>
            <p className="eyebrow mb-4">Documentation</p>
            <div className="space-y-6">
              {Object.entries(sections).map(([section, items]) => (
                <div key={section} className="space-y-2">
                  <p className="font-mono text-[0.68rem] uppercase tracking-[0.18em] text-[var(--muted)]">
                    {section}
                  </p>
                  <nav className="flex flex-col gap-2">
                    {items.map((doc) => (
                      <Link
                        key={doc.slug}
                        href={`/docs/${doc.slug}`}
                        className="docs-sidebar-link font-copy text-[1.02rem]"
                      >
                        {doc.title}
                      </Link>
                    ))}
                  </nav>
                </div>
              ))}
            </div>
          </div>
        </div>
      </aside>
      <main className="min-w-0 flex-1">{children}</main>
    </div>
  );
}
