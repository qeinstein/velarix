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
        <h1 className="font-display text-4xl md:text-5xl leading-tight tracking-[-0.05em] mb-4">
          {doc.title}
        </h1>
        {doc.description && (
          <p className="copy-tone font-copy text-xl leading-8">
            {doc.description}
          </p>
        )}
      </div>

      <div className="font-copy text-lg leading-8">
        <ReactMarkdown 
          rehypePlugins={[rehypeHighlight]}
          components={{
            h1: ({node, ...props}) => <h1 className="font-display text-3xl tracking-[-0.04em] mt-10 mb-4" {...props} />,
            h2: ({node, ...props}) => <h2 className="font-display text-2xl tracking-[-0.04em] mt-10 mb-4" {...props} />,
            h3: ({node, ...props}) => <h3 className="font-display text-xl tracking-[-0.04em] mt-8 mb-4" {...props} />,
            p: ({node, ...props}) => <p className="mb-6" {...props} />,
            ul: ({node, ...props}) => <ul className="list-disc pl-6 mb-6 space-y-2" {...props} />,
            ol: ({node, ...props}) => <ol className="list-decimal pl-6 mb-6 space-y-2" {...props} />,
            li: ({node, ...props}) => <li className="" {...props} />,
            a: ({node, ...props}) => <a className="text-link no-underline hover:border-foreground" {...props} />,
            code: ({node, inline, className, children, ...props}: any) => {
              if (inline) {
                return <code className="font-mono text-[0.88em] bg-[var(--panel)] px-1.5 py-0.5 rounded" {...props}>{children}</code>;
              }
              return <code className={className} {...props}>{children}</code>;
            },
            pre: ({node, ...props}) => <pre className="font-mono text-[0.88em] bg-[var(--code-bg)] border border-[var(--line)] rounded-xl p-4 overflow-x-auto mb-6" {...props} />,
          }}
        >
          {doc.content}
        </ReactMarkdown>
      </div>

      <div className="section-rule mt-16 flex flex-col sm:flex-row gap-6 justify-between items-center">
        {prevDoc ? (
          <Link href={`/docs/${prevDoc.slug}`} className="surface p-5 rounded-lg w-full sm:w-[48%] transition-all hover:border-foreground group">
            <span className="eyebrow block mb-2">Previous</span>
            <span className="font-copy text-xl group-hover:text-foreground transition-colors">{prevDoc.title}</span>
          </Link>
        ) : (
          <div className="w-full sm:w-[48%]" />
        )}

        {nextDoc ? (
          <Link href={`/docs/${nextDoc.slug}`} className="surface p-5 rounded-lg w-full sm:w-[48%] text-right transition-all hover:border-foreground group">
            <span className="eyebrow block mb-2">Next</span>
            <span className="font-copy text-xl group-hover:text-foreground transition-colors">{nextDoc.title}</span>
          </Link>
        ) : (
          <div className="w-full sm:w-[48%]" />
        )}
      </div>
    </article>
  );
}
