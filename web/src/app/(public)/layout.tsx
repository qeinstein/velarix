import ThemeToggle from "../../components/ThemeToggle";

export default function PublicLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="site-shell">
      <header className="site-nav">
        <a href="/" className="site-brand">
          Velarix
        </a>
        <div className="site-actions">
          <nav className="site-links" aria-label="Primary">
            <a href="/docs">Docs</a>
            <a
              href="https://github.com/qeinstein/velarix"
              target="_blank"
              rel="noreferrer"
            >
              GitHub
            </a>
            <a href="/login">Login</a>
          </nav>
          <ThemeToggle />
        </div>
      </header>
      {children}
    </div>
  );
}
