import { notFound } from "next/navigation";

import { DocsPageView } from "../../../components/docs/docs-page-view";
import { getAllDocSlugs, getDocBySlug } from "../../../components/docs/docs-data";

export function generateStaticParams() {
  return getAllDocSlugs().map((slug) => ({ slug }));
}

export default async function DocSlugPage({
  params,
}: {
  params: Promise<{ slug: string[] }>;
}) {
  const { slug } = await params;
  const page = getDocBySlug(slug);

  if (!page) {
    notFound();
  }

  return <DocsPageView page={page} />;
}
