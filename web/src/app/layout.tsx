import type { Metadata } from "next";
import Script from "next/script";
import "./globals.css";
import ThemeToggle from "../components/ThemeToggle";

export const metadata: Metadata = {
  title: "Velarix | Correctable Memory For AI Agents",
  description: "A causal memory layer that lets AI agents prune stale reasoning when facts change.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const themeScript = `
    try {
      var key = 'velarix-theme';
      var stored = localStorage.getItem(key);
      var theme = stored === 'dark' || stored === 'light'
        ? stored
        : (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
      document.documentElement.dataset.theme = theme;
      document.documentElement.style.colorScheme = theme;
    } catch (error) {}
  `;

  return (
    <html lang="en" data-theme="light" suppressHydrationWarning>
      <body>
        <Script id="velarix-theme-init" strategy="beforeInteractive">
          {themeScript}
        </Script>
        <div className="site-shell">
          <header className="site-nav">
            <a href="/" className="site-brand">
              Velarix
            </a>
            <div className="site-actions">
              <nav className="site-links" aria-label="Primary">
                <a href="/docs">Docs</a>
                <a href="https://github.com/qeinstein/velarix" target="_blank" rel="noreferrer">
                  GitHub
                </a>
                <a href="/login">Login</a>
              </nav>
              <ThemeToggle />
            </div>
          </header>
          {children}
        </div>
      </body>
    </html>
  );
}
