import { getDocBySlug, getDocsList } from "@/lib/docs";
import { notFound } from "next/navigation";
import { MDXRemote } from "next-mdx-remote/rsc";
import rehypeHighlight from "rehype-highlight";
import "highlight.js/styles/github-dark.css";
import Link from "next/link";
import { mdxComponents } from "@/components/mdx";

export async function generateStaticParams() {
  const docs = getDocsList();
  return docs.map((doc) => ({ slug: doc.slug }));
}

export default function DocPage({ params }: { params: { slug: string } }) {
  const doc = getDocBySlug(params.slug);
  if (!doc) notFound();

  const docs = getDocsList();
  const currentIndex = docs.findIndex((d) => d.slug === params.slug);
  const prevDoc = currentIndex > 0 ? docs[currentIndex - 1] : null;
  const nextDoc =
    currentIndex < docs.length - 1 ? docs[currentIndex + 1] : null;

  return (
    <article className="doc-article">
      <header className="doc-header">
        <p className="doc-section-label">{doc.section}</p>
        <h1 className="doc-title">{doc.title}</h1>
        {doc.description && (
          <p className="doc-description">{doc.description}</p>
        )}
      </header>

      <div className="doc-body">
        <MDXRemote
          source={doc.content}
          components={mdxComponents}
          options={{
            mdxOptions: {
              rehypePlugins: [rehypeHighlight],
            },
          }}
        />
      </div>

      <nav className="doc-pagination">
        {prevDoc ? (
          <Link
            href={`/docs/${prevDoc.slug}`}
            className="doc-pagination-item is-prev"
          >
            <span className="doc-pagination-label">Previous</span>
            <span className="doc-pagination-title">{prevDoc.title}</span>
          </Link>
        ) : (
          <div />
        )}
        {nextDoc ? (
          <Link
            href={`/docs/${nextDoc.slug}`}
            className="doc-pagination-item is-next"
          >
            <span className="doc-pagination-label">Next</span>
            <span className="doc-pagination-title">{nextDoc.title}</span>
          </Link>
        ) : (
          <div />
        )}
      </nav>
    </article>
  );
}
