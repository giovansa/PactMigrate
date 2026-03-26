import * as React from "react";

import { APP_ICON_URL } from "../../constants";
import { cn } from "../../lib/utils";
import { Button } from "../ui/button";

export type NavKey = "migrations" | "more";

const NAV_ITEMS: { id: NavKey; label: string; description: string }[] = [
  { id: "migrations", label: "Migrations", description: "Plans, runs, drift" },
  { id: "more", label: "More", description: "Coming soon" },
];

type AppLayoutProps = {
  activeNav: NavKey;
  onNavigate: (key: NavKey) => void;
  onLogout: () => void;
  onRefresh: () => void;
  loading: boolean;
  children: React.ReactNode;
};

export function AppLayout({ activeNav, onNavigate, onLogout, onRefresh, loading, children }: AppLayoutProps) {
  const meta = NAV_ITEMS.find((n) => n.id === activeNav);

  return (
    <div className="grid min-h-screen grid-cols-[14rem_1fr] grid-rows-[auto_1fr] bg-background text-foreground">
      {/* Top row: one shared baseline — both cells stretch to the same height */}
      <div className="flex items-center gap-3 border-b border-r border-border bg-muted/40 px-4 py-4">
        <img src={APP_ICON_URL} alt="" className="h-10 w-10 shrink-0 object-contain" aria-hidden />
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold">PactMigrate</div>
          <div className="truncate text-xs text-muted-foreground">Console</div>
        </div>
      </div>

      <header className="flex min-h-0 flex-col gap-3 border-b border-border px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold tracking-tight">{meta?.label ?? "PactMigrate"}</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {activeNav === "migrations"
              ? "Database migration status across environments with run history."
              : "Space for additional product areas as PactMigrate grows."}
          </p>
        </div>
        {activeNav === "migrations" ? (
          <div className="shrink-0">
            <Button variant="secondary" onClick={() => void onRefresh()} disabled={loading}>
              {loading ? "Loading..." : "Refresh"}
            </Button>
          </div>
        ) : null}
      </header>

      {/* Body */}
      <aside className="flex min-h-0 flex-col border-r border-border bg-muted/40">
        <nav className="flex flex-1 flex-col gap-1 p-3" aria-label="Main">
          {NAV_ITEMS.map((item) => {
            const active = activeNav === item.id;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => onNavigate(item.id)}
                className={cn(
                  "w-full rounded-md px-3 py-2.5 text-left text-sm transition-colors",
                  active
                    ? "bg-primary text-primary-foreground shadow-sm"
                    : "text-foreground hover:bg-muted"
                )}
              >
                <div className="font-medium">{item.label}</div>
                <div
                  className={cn(
                    "mt-0.5 text-xs",
                    active ? "text-primary-foreground/85" : "text-muted-foreground"
                  )}
                >
                  {item.description}
                </div>
              </button>
            );
          })}
        </nav>

        <div className="border-t border-border p-3">
          <Button variant="outline" className="w-full" onClick={onLogout} disabled={loading}>
            Logout
          </Button>
        </div>
      </aside>

      <main className="min-h-0 overflow-auto p-6">{children}</main>
    </div>
  );
}
