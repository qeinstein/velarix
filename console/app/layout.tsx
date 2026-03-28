import type { Metadata } from "next";
import { Geist_Mono, Inter, Lora } from "next/font/google";

import "./globals.css";

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
  display: "swap",
});

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
  display: "swap",
});

const lora = Lora({
  variable: "--font-lora",
  subsets: ["latin"],
  display: "swap",
  weight: ["400", "500"],
  style: ["normal", "italic"],
});

export const metadata: Metadata = {
  title: "Velarix",
  description:
    "The logic layer for AI agents. Deterministic reasoning, real-time causal validation, and kernel-level auditability.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${geistMono.variable} ${lora.variable} ${inter.variable}`}
      suppressHydrationWarning
    >
      <body
        className="bg-background text-text-primary font-sans"
        suppressHydrationWarning
      >
        {children}
      </body>
    </html>
  );
}
