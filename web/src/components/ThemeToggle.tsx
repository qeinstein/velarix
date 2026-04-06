"use client";

import { useEffect, useState } from "react";

type Theme = "light" | "dark";

const STORAGE_KEY = "velarix-theme";

function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
  localStorage.setItem(STORAGE_KEY, theme);
}

export default function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>("light");

  useEffect(() => {
    const currentTheme = document.documentElement.dataset.theme;
    if (currentTheme === "dark" || currentTheme === "light") {
      setTheme(currentTheme);
      return;
    }

    const fallback = window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    setTheme(fallback);
    applyTheme(fallback);
  }, []);

  const toggleTheme = () => {
    const nextTheme = theme === "dark" ? "light" : "dark";
    setTheme(nextTheme);
    applyTheme(nextTheme);
  };

  return (
    <div className="theme-toggle__meta">
      <span className="theme-toggle__note">Paper / Ink</span>
      <button
        type="button"
        aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} theme`}
        aria-pressed={theme === "dark"}
        className="theme-toggle"
        onClick={toggleTheme}
      >
        <span className={`theme-toggle__thumb ${theme === "dark" ? "is-dark" : ""}`} />
        <span className="theme-toggle__label">Wht</span>
        <span className="theme-toggle__label">Blk</span>
      </button>
    </div>
  );
}
