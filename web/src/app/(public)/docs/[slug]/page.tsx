import { getDocBySlug, getDocsList, extractHeadings } from "@/lib/docs";
import { notFound } from "next/navigation";
import ReactMarkdown from "react-markdown";
import rehypeHighlight from "rehype-highlight";
import "highlight.js/styles/github-dark.css";
import Link from "next/link";
import remarkSlug from "remark-slug";

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

  const headings = extractHeadings(doc.content);

  return (
    <div className="flex xl:gap-8">
      {/* Main Content */}
      <article className="min-w-0 flex-1 space-y-12 xl:max-w-3xl pb-24">
        {/* Header */}
        <header className="space-y-4">
          <h1 className="font-display text-4xl tracking-tight text-zinc-100 sm:text-5xl">
            {doc.title}
          </h1>
          {doc.description && (
            <p className="text-xl leading-8 text-zinc-400 font-copy">
              {doc.description}
            </p>
          )}
        </header>

        {/* Prose Markdown Wrapper */}
        <div className="prose prose-invert prose-zinc max-w-none 
          prose-headings:font-display prose-headings:tracking-tight prose-headings:text-zinc-100
          prose-a:text-indigo-400 prose-a:no-underline hover:prose-a:underline hover:prose-a:text-indigo-300
          prose-p:text-zinc-400 prose-p:leading-8
          prose-strong:text-zinc-200
          prose-li:text-zinc-400
          prose-code:text-zinc-300 prose-code:bg-white/5 prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded-md prose-code:font-mono prose-code:text-sm prose-code:before:content-none prose-code:after:content-none
          prose-pre:bg-[#0d0d0d] prose-pre:border prose-pre:border-white/10 prose-pre:rounded-xl prose-pre:shadow-2xl prose-pre:max-w-[calc(100vw-2rem)]
          sm:prose-pre:max-w-none
          font-copy text-lg
        ">
          <ReactMarkdown
            rehypePlugins={[rehypeHighlight]}
            components={{
              h2: ({ node, children, ...props }) => {
                const id = String(children).toLowerCase().replace(/[^\w]+/g, '-').replace(/(^-|-$)/g, '');
                return <h2 id={id} {...props}>{children}</h2>;
              },
              h3: ({ node, children, ...props }) => {
                const id = String(children).toLowerCase().replace(/[^\w]+/g, '-').replace(/(^-|-$)/g, '');
                return <h3 id={id} {...props}>{children}</h3>;
              },
            }}
          >
            {doc.content}
          </ReactMarkdown>
        </div>

        {/* Navigation Footer */}
        <div className="mt-16 flex flex-col items-center justify-between gap-4 border-t border-white/10 pt-10 sm:flex-row">
          {prevDoc ? (
            <Link
              href={`/docs/${prevDoc.slug}`}
              className="group flex w-full flex-col justify-center rounded-xl border border-white/5 bg-white/5 p-6 transition-all hover:bg-white/10 hover:border-white/20 sm:w-[48%]"
            >
              <span className="text-xs uppercase tracking-wider text-zinc-500 mb-2 font-semibold transition-colors group-hover:text-zinc-400">Previous</span>
              <span className="font-copy text-lg font-medium text-zinc-300 transition-colors group-hover:text-zinc-100">
                {prevDoc.title}
              </span>
            </Link>
          ) : (
            <div className="w-full sm:w-[48%]" />
          )}

          {nextDoc ? (
            <Link
              href={`/docs/${nextDoc.slug}`}
              className="group flex w-full flex-col justify-center text-right rounded-xl border border-white/5 bg-white/5 p-6 transition-all hover:bg-white/10 hover:border-white/20 sm:w-[48%]"
            >
              <span className="text-xs uppercase tracking-wider text-zinc-500 mb-2 font-semibold transition-colors group-hover:text-zinc-400">Next</span>
              <span className="font-copy text-lg font-medium text-zinc-300 transition-colors group-hover:text-zinc-100">
                {nextDoc.title}
              </span>
            </Link>
          ) : (
            <div className="w-full sm:w-[48%]" />
          )}
        </div>
      </article>

      {/* Right Sidebar TOC */}
      {headings.length > 0 && (
        <aside className="hidden xl:block xl:w-64 flex-shrink-0">
          <div className="sticky top-24">
            <h4 className="text-sm font-semibold tracking-wider text-zinc-100 uppercase mb-4">On this page</h4>
            <nav className="flex flex-col gap-2.5">
              {headings.map((heading, i) => (
                <a
                  key={i}
                  href={`#${heading.id}`}
                  className={`text-sm tracking-tight transition-colors hover:text-zinc-100 ${
                    heading.level === 3 ? "ml-4 text-zinc-500" : "text-zinc-400 font-medium"
                  }`}
                >
                  {heading.text}
                </a>
              ))}
            </nav>
          </div>
        </aside>
      )}
    </div>
  );
}
