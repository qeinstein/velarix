"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import Link from "next/link";
import ThemeToggle from "@/components/ThemeToggle";
import { apiFetch } from "@/lib/api";

const navItems = [
  { href: "/dashboard", label: "Overview", exact: true },
  { href: "/dashboard/projects", label: "Projects", exact: false },
  { href: "/dashboard/team", label: "Team", exact: true },
  { href: "/dashboard/keys", label: "API Keys", exact: true },
  { href: "/dashboard/settings", label: "Settings", exact: true },
];

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const [email, setEmail] = useState<string>("");

  useEffect(() => {
    apiFetch("/v1/me")
      .then((r) => {
        if (r.status === 401) {
          router.push("/login");
          return null;
        }
        return r.ok ? r.json() : null;
      })
      .then((data) => {
        if (data?.email) setEmail(data.email);
      })
      .catch(() => {});
  }, [router]);

  const handleSignOut = async () => {
    await apiFetch("/v1/auth/logout", { method: "POST" });
    localStorage.removeItem("vlx_token");
    router.push("/");
  };

  const isActive = (item: { href: string; exact: boolean }) => {
    if (item.exact) return pathname === item.href;
    return pathname.startsWith(item.href);
  };

  return (
    <div className="dash-root">
      <aside className="dash-sidebar">
        <div className="dash-sidebar-header">
          <a href="/" className="dash-sidebar-brand">
            Velarix
          </a>
          <ThemeToggle />
        </div>

        <nav className="dash-sidebar-nav">
          <p className="dash-nav-label">Workspace</p>
          {navItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={`dash-nav-item ${isActive(item) ? "active" : ""}`}
            >
              {item.label}
            </Link>
          ))}
        </nav>

        <div className="dash-sidebar-footer">
          <div className="dash-sidebar-user">
            {email && <span className="dash-sidebar-email">{email}</span>}
            <button
              onClick={handleSignOut}
              className="text-link mt-2 w-fit"
              style={{ fontSize: "0.7rem" }}
            >
              Sign Out
            </button>
          </div>
        </div>
      </aside>

      <main className="dash-main">{children}</main>
    </div>
  );
}
