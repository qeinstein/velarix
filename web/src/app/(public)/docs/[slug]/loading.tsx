export default function DocLoading() {
  return (
    <article className="space-y-10 animate-pulse">
      <div className="space-y-4">
        <div className="h-10 w-2/3 rounded-lg bg-[var(--panel)]" />
        <div className="h-5 w-1/2 rounded bg-[var(--panel)]" />
      </div>

      <div className="space-y-4">
        {[...Array(3)].map((_, i) => (
          <div key={i} className="space-y-2">
            <div className="h-4 rounded bg-[var(--panel)]" />
            <div className="h-4 w-11/12 rounded bg-[var(--panel)]" />
            <div className="h-4 w-4/5 rounded bg-[var(--panel)]" />
          </div>
        ))}

        <div className="h-32 rounded-xl bg-[var(--panel)]" />

        {[...Array(4)].map((_, i) => (
          <div key={i} className="space-y-2">
            <div className="h-4 rounded bg-[var(--panel)]" />
            <div className="h-4 w-5/6 rounded bg-[var(--panel)]" />
            <div className="h-4 w-3/4 rounded bg-[var(--panel)]" />
          </div>
        ))}
      </div>

      <div className="section-rule mt-16 flex flex-col items-center justify-between gap-6 sm:flex-row">
        <div className="h-20 w-full rounded-lg bg-[var(--panel)] sm:w-[48%]" />
        <div className="h-20 w-full rounded-lg bg-[var(--panel)] sm:w-[48%]" />
      </div>
    </article>
  );
}
