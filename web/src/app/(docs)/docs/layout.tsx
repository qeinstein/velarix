import { getDocsList } from "@/lib/docs";
import { DocsShell } from "./DocsShell";

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const docs = getDocsList();
  return <DocsShell docs={docs}>{children}</DocsShell>;
}
