import { getDocsList } from "@/lib/docs";
import Link from "next/link";

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const docs = getDocsList();

  return (
    <div className="flex flex-col gap-8 pb-24 pt-8 md:flex-row">
      <aside className="w-full flex-shrink-0 md:w-64">
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
      <main className="min-w-0 flex-1">{children}</main>
    </div>
  );
}
