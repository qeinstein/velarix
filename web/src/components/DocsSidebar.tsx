"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { DocMetadata } from "@/lib/docs";

export function DocsSidebar({ docs }: { docs: DocMetadata[] }) {
  const pathname = usePathname();

  return (
    <nav className="flex flex-col gap-1">
      {docs.map((doc) => {
        const href = `/docs/${doc.slug}`;
        const isActive = pathname === href;

        return (
          <Link
            key={doc.slug}
            href={href}
            className={`group relative flex items-center rounded-md px-3 py-2 text-sm font-medium transition-colors ${
              isActive
                ? "bg-white/10 text-white"
                : "text-zinc-400 hover:bg-white/5 hover:text-zinc-100"
            }`}
          >
            {isActive && (
              <span className="absolute left-0 bottom-1.5 top-1.5 w-1 rounded-r-md bg-white" />
            )}
            {doc.title}
          </Link>
        );
      })}
    </nav>
  );
}
