"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";
import { ApiError, request } from "../lib/api";
import { useTheme } from "../hooks/useTheme";
import type { App } from "../types";
import { NewAppModal } from "./NewAppModal";

type DashboardContextValue = {
  apps: App[];
  refreshApps: () => Promise<App[]>;
  openNewProject: () => void;
  setFeedback: (message: string, kind?: "success" | "error") => void;
};

const DashboardContext = createContext<DashboardContextValue | null>(null);

export function useDashboard() {
  const value = useContext(DashboardContext);
  if (!value) throw new Error("useDashboard must be used inside DashboardShell");
  return value;
}

const navItems = [
  { href: "/dashboard/projects", label: "Projetos" },
  { href: "/dashboard/deployments", label: "Deployments" },
  { href: "/dashboard/logs", label: "Logs" },
  { href: "/dashboard/metrics", label: "Métricas" },
];

function pageTitle(pathname: string) {
  if (pathname.startsWith("/dashboard/deployments")) return "Deployments";
  if (pathname.startsWith("/dashboard/logs")) return "Logs";
  if (pathname.startsWith("/dashboard/metrics")) return "Métricas";
  if (/^\/dashboard\/projects\/.+/.test(pathname)) return "Detalhes do projeto";
  return "Projetos";
}

export function DashboardShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { theme, toggleTheme } = useTheme();
  const [apps, setApps] = useState<App[]>([]);
  const [ready, setReady] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [showNewApp, setShowNewApp] = useState(false);
  const [newAppName, setNewAppName] = useState("");
  const [creatingApp, setCreatingApp] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);
  const [feedback, setFeedbackState] = useState<{ message: string; kind: "success" | "error" } | null>(null);
  const closeNewApp = useCallback(() => setShowNewApp(false), []);

  const setFeedback = useCallback((message: string, kind: "success" | "error" = "success") => {
    setFeedbackState(message ? { message, kind } : null);
  }, []);

  const refreshApps = useCallback(async () => {
    const nextApps = await request<App[]>("/apps");
    setApps(nextApps);
    return nextApps;
  }, []);

  useEffect(() => {
    let active = true;
    const timer = window.setTimeout(() => {
      refreshApps()
        .catch((cause: unknown) => {
          if (cause instanceof ApiError && cause.status === 401) {
            router.replace("/login");
            return;
          }
          if (active) setFeedback(cause instanceof Error ? cause.message : "Não foi possível conectar à API.", "error");
        })
        .finally(() => { if (active) setReady(true); });
    }, 0);
    return () => { active = false; window.clearTimeout(timer); };
  }, [refreshApps, router, setFeedback]);

  useEffect(() => {
    if (!ready) return;
    const timer = window.setInterval(() => refreshApps().catch(() => undefined), 5000);
    return () => window.clearInterval(timer);
  }, [ready, refreshApps]);

  async function logout() {
    setLoggingOut(true);
    try {
      await request("/auth/logout", { method: "POST" });
      router.replace("/");
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível encerrar a sessão.", "error");
    } finally {
      setLoggingOut(false);
    }
  }

  async function createApp(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!newAppName.trim()) return;
    setCreatingApp(true);
    try {
      const app = await request<App>("/apps", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: newAppName.trim() }),
      });
      setShowNewApp(false);
      setNewAppName("");
      await refreshApps();
      setFeedback(`Projeto ${app.name} criado.`);
      router.push(`/dashboard/projects/${encodeURIComponent(app.name)}`);
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível criar o projeto.", "error");
    } finally {
      setCreatingApp(false);
    }
  }

  const contextValue = useMemo(() => ({ apps, refreshApps, openNewProject: () => setShowNewApp(true), setFeedback }), [apps, refreshApps, setFeedback]);

  if (!ready) return <main className="loading-screen">Iniciando o painel…</main>;

  return (
    <DashboardContext.Provider value={contextValue}>
      <main className="app-shell" data-theme={theme}>
        <div className="technical-backdrop" aria-hidden="true">
          <svg className="technical-canvas" viewBox="0 0 1600 1000" preserveAspectRatio="none">
            <defs>
              <pattern id="control-grid" width="80" height="80" patternUnits="userSpaceOnUse">
                <path d="M 80 0 L 0 0 0 80" className="technical-grid-line" />
              </pattern>
            </defs>
            <rect width="1600" height="1000" fill="url(#control-grid)" />
            <g className="technical-circuit">
              <path d="M42 174H294V112H520V208H744" />
              <path d="M906 92V246H1180V174H1538" />
              <path d="M70 724H238V814H492V758H678" />
              <path d="M1012 664H1188V760H1398V706H1570" />
              <path d="M356 1000V884H592V930H842" />
              <path d="M1308 0V126H1134V56H980" />
            </g>
            <g className="technical-circuit technical-circuit-accent">
              <path d="M154 350H306V296H424" />
              <path d="M1214 436H1338V388H1490" />
            </g>
            <g className="technical-nodes">
              <circle cx="294" cy="174" r="3" />
              <circle cx="520" cy="112" r="3" />
              <circle cx="744" cy="208" r="3" />
              <circle cx="1180" cy="246" r="3" />
              <circle cx="238" cy="724" r="3" />
              <circle cx="492" cy="814" r="3" />
              <circle cx="1188" cy="664" r="3" />
              <circle cx="1398" cy="760" r="3" />
            </g>
            <g className="technical-nodes technical-nodes-accent">
              <circle cx="306" cy="350" r="3" />
              <circle cx="1338" cy="436" r="3" />
            </g>
            <g className="technical-fragments">
              <rect x="104" y="430" width="122" height="1" />
              <rect x="104" y="442" width="74" height="1" />
              <rect x="104" y="454" width="96" height="1" />
              <rect x="1298" y="286" width="158" height="1" />
              <rect x="1298" y="298" width="88" height="1" />
              <rect x="1298" y="310" width="126" height="1" />
              <rect x="772" y="846" width="42" height="2" />
              <rect x="824" y="846" width="94" height="2" />
            </g>
          </svg>
        </div>
        <button className="mobile-menu-button" onClick={() => setMenuOpen((open) => !open)} aria-expanded={menuOpen} aria-controls="dashboard-sidebar">Menu</button>
        <aside id="dashboard-sidebar" className={`sidebar glass ${menuOpen ? "open" : ""}`}>
          <Link className="brand" href="/dashboard/projects"><span className="brand-mark">M</span><span>MiniPaaS</span></Link>
          <nav aria-label="Navegação principal">
            {navItems.map((item) => {
              const active = pathname === item.href || (item.href.endsWith("projects") && pathname.startsWith(`${item.href}/`));
              return <Link key={item.href} className={`nav-item ${active ? "active" : ""}`} href={item.href} onClick={() => setMenuOpen(false)}>{item.label}</Link>;
            })}
          </nav>
          <div className="sidebar-footer"><span className="status-dot healthy" /> API conectada</div>
        </aside>

        <section className="workspace">
          <header className="dashboard-topbar glass">
            <div className="breadcrumb"><span>Control plane</span><strong>{pageTitle(pathname)}</strong></div>
            <div className="topbar-actions">
              <button className="button primary compact-action" onClick={() => setShowNewApp(true)}>Nova aplicação</button>
              <button className="theme-button" onClick={toggleTheme}>{theme === "dark" ? "Modo claro" : "Modo escuro"}</button>
              <button className="theme-button" onClick={logout} disabled={loggingOut}>{loggingOut ? "Saindo…" : "Sair"}</button>
            </div>
          </header>

          {feedback && <div className={`feedback ${feedback.kind}`} role="status"><span>{feedback.message}</span><button onClick={() => setFeedbackState(null)} aria-label="Fechar aviso">Fechar</button></div>}
          <div className="page-content">{children}</div>
        </section>

        {showNewApp && <NewAppModal name={newAppName} creating={creatingApp} onNameChange={setNewAppName} onClose={closeNewApp} onSubmit={createApp} />}
      </main>
    </DashboardContext.Provider>
  );
}
