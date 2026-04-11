import { getDocBySlug, getDocsList } from "@/lib/docs";
import { notFound } from "next/navigation";
import ReactMarkdown from "react-markdown";
import rehypeHighlight from "rehype-highlight";
import "highlight.js/styles/github-dark.css";
import Link from "next/link";

export async function generateStaticParams() {
  const docs = getDocsList();
  return docs.map((doc) => ({
    slug: doc.slug,
  }));
}

export default function DocPage({ params }: { params: { slug: string } }) {
  const doc = getDocBySlug(params.slug);

  if (!doc) {
    notFound();
  }

  const docs = getDocsList();
  const currentIndex = docs.findIndex((d) => d.slug === params.slug);
  const prevDoc = currentIndex > 0 ? docs[currentIndex - 1] : null;
  const nextDoc = currentIndex < docs.length - 1 ? docs[currentIndex + 1] : null;

  return (
    <article className="space-y-10">
      <div>
        <h1 className="font-display mb-4 text-4xl leading-tight tracking-[-0.05em] md:text-5xl">
          {doc.title}
        </h1>
        {doc.description && (
          <p className="copy-tone font-copy text-xl leading-8">{doc.description}</p>
        )}
      </div>

      <div className="font-copy text-lg leading-8">
        <ReactMarkdown
          rehypePlugins={[rehypeHighlight]}
          components={{
            h1: ({ node, ...props }) => (
              <h1
                className="font-display mb-4 mt-10 text-3xl tracking-[-0.04em]"
                {...props}
              />
            ),
            h2: ({ node, ...props }) => (
              <h2
                className="font-display mb-4 mt-10 text-2xl tracking-[-0.04em]"
                {...props}
              />
            ),
            h3: ({ node, ...props }) => (
              <h3
                className="font-display mb-4 mt-8 text-xl tracking-[-0.04em]"
                {...props}
              />
            ),
            p: ({ node, ...props }) => <p className="mb-6" {...props} />,
            ul: ({ node, ...props }) => (
              <ul className="mb-6 list-disc space-y-2 pl-6" {...props} />
            ),
            ol: ({ node, ...props }) => (
              <ol className="mb-6 list-decimal space-y-2 pl-6" {...props} />
            ),
            li: ({ node, ...props }) => <li className="" {...props} />,
            a: ({ node, ...props }) => (
              <a
                className="text-link no-underline hover:border-foreground"
                {...props}
              />
            ),
            code: ({
              node,
              inline,
              className,
              children,
              ...props
            }: any) => {
              if (inline) {
                return (
                  <code
                    className="rounded bg-[var(--panel)] px-1.5 py-0.5 font-mono text-[0.88em]"
                    {...props}
                  >
                    {children}
                  </code>
                );
              }
              return (
                <code className={className} {...props}>
                  {children}
                </code>
              );
            },
            pre: ({ node, ...props }) => (
              <pre
                className="mb-6 overflow-x-auto rounded-xl border border-[var(--line)] bg-[var(--code-bg)] p-4 font-mono text-[0.88em]"
                {...props}
              />
            ),
          }}
        >
          {doc.content}
        </ReactMarkdown>
      </div>

      <div className="section-rule mt-16 flex flex-col items-center justify-between gap-6 sm:flex-row">
        {prevDoc ? (
          <Link
            href={`/docs/${prevDoc.slug}`}
            className="surface group w-full rounded-lg p-5 transition-all hover:border-foreground sm:w-[48%]"
          >
            <span className="eyebrow mb-2 block">Previous</span>
            <span className="font-copy text-xl transition-colors group-hover:text-foreground">
              {prevDoc.title}
            </span>
          </Link>
        ) : (
          <div className="w-full sm:w-[48%]" />
        )}

        {nextDoc ? (
          <Link
            href={`/docs/${nextDoc.slug}`}
            className="surface group w-full rounded-lg p-5 text-right transition-all hover:border-foreground sm:w-[48%]"
          >
            <span className="eyebrow mb-2 block">Next</span>
            <span className="font-copy text-xl transition-colors group-hover:text-foreground">
              {nextDoc.title}
            </span>
          </Link>
        ) : (
          <div className="w-full sm:w-[48%]" />
        )}
      </div>
    </article>
  );
}
