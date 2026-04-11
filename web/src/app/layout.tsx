import type { Metadata } from "next";
import Script from "next/script";
import "./globals.css";

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
        {children}
      </body>
    </html>
  );
}
