import { MDXComponents } from "mdx/types";
import { Callout } from "./Callout";

export const mdxComponents: MDXComponents = {
  // Custom components available in every .mdx file
  Callout,

  // HTML element overrides
  h1: ({ children, ...props }) => (
    <h1
      className="font-display mb-4 mt-10 text-3xl tracking-[-0.04em] first:mt-0"
      {...props}
    >
      {children}
    </h1>
  ),
  h2: ({ children, ...props }) => (
    <h2
      className="font-display mb-4 mt-10 text-2xl tracking-[-0.04em]"
      {...props}
    >
      {children}
    </h2>
  ),
  h3: ({ children, ...props }) => (
    <h3
      className="font-display mb-4 mt-8 text-xl tracking-[-0.04em]"
      {...props}
    >
      {children}
    </h3>
  ),
  p: ({ children, ...props }) => (
    <p className="mb-6" {...props}>
      {children}
    </p>
  ),
  ul: ({ children, ...props }) => (
    <ul className="mb-6 list-disc space-y-2 pl-6" {...props}>
      {children}
    </ul>
  ),
  ol: ({ children, ...props }) => (
    <ol className="mb-6 list-decimal space-y-2 pl-6" {...props}>
      {children}
    </ol>
  ),
  a: ({ children, ...props }) => (
    <a className="text-link no-underline hover:border-foreground" {...props}>
      {children}
    </a>
  ),
  code: ({ children, className, ...props }) => {
    // Inline code (no language class)
    if (!className) {
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
  pre: ({ children, ...props }) => (
    <pre
      className="mb-6 overflow-x-auto rounded-xl border border-[var(--line)] bg-[var(--code-bg)] p-4 font-mono text-[0.88em]"
      {...props}
    >
      {children}
    </pre>
  ),
  table: ({ children, ...props }) => (
    <div className="mb-6 overflow-x-auto">
      <table
        className="w-full border-collapse font-copy text-base"
        {...props}
      >
        {children}
      </table>
    </div>
  ),
  th: ({ children, ...props }) => (
    <th
      className="border border-[var(--line)] bg-[var(--panel)] px-4 py-2 text-left font-mono text-[0.75rem] uppercase tracking-[0.1em] text-[var(--muted)]"
      {...props}
    >
      {children}
    </th>
  ),
  td: ({ children, ...props }) => (
    <td
      className="border border-[var(--line)] px-4 py-2"
      {...props}
    >
      {children}
    </td>
  ),
  blockquote: ({ children, ...props }) => (
    <blockquote
      className="mb-6 border-l-2 border-[var(--line)] pl-4 text-[var(--muted)] italic"
      {...props}
    >
      {children}
    </blockquote>
  ),
  hr: () => <hr className="section-rule my-10" />,
};
