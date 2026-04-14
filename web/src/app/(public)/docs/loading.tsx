export default function DocsLoading() {
  return (
    <div className="flex flex-col gap-8 pb-24 pt-8 md:flex-row animate-pulse">
      <aside className="w-full flex-shrink-0 md:w-72">
        <div className="sticky top-24 space-y-6">
          <div className="h-4 w-24 rounded bg-[var(--panel)]" />
          <div className="space-y-3">
            {[...Array(6)].map((_, i) => (
              <div key={i} className="h-4 w-40 rounded bg-[var(--panel)]" />
            ))}
          </div>
        </div>
      </aside>
      <main className="min-w-0 flex-1" />
    </div>
  );
}
