import { getDocsList } from "@/lib/docs";
import Link from "next/link";

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const docs = getDocsList();

  return (
    <div className="flex flex-col md:flex-row gap-8 pt-8 pb-24">
      {/* Sidebar Navigation */}
      <aside className="w-full md:w-64 flex-shrink-0">
        <div className="sticky top-24 space-y-6">
          <div>
            <p className="eyebrow mb-4">Documentation</p>
            <nav className="flex flex-col gap-2">
              {docs.map((doc) => (
                <Link
                  key={doc.slug}
                  href={`/docs/${doc.slug}`}
                  className="docs-sidebar-link font-copy text-[1.05rem]"
                >
                  {doc.title}
                </Link>
              ))}
            </nav>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 min-w-0">
        {children}
      </main>
    </div>
  );
}
