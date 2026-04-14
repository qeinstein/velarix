import { getDocsList } from "@/lib/docs";
import Link from "next/link";
import { DocsSidebar } from "@/components/DocsSidebar";

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
            <p className="eyebrow mb-4 pl-3 font-semibold text-zinc-100">
              Documentation
            </p>
            <DocsSidebar docs={docs} />
          </div>
        </div>
      </aside>
      <main className="min-w-0 flex-1">{children}</main>
    </div>
  );
}
