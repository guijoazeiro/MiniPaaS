"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, request } from "./lib/api";
import { useApps } from "./hooks/useApps";
import { useLogStream } from "./hooks/useLogStream";
import { Landing } from "./components/Landing";
import { LoginScreen } from "./components/LoginScreen";
import { AppList } from "./components/AppList";
import { DeployPanel } from "./components/DeployPanel";
import { DeploymentList } from "./components/DeploymentList";
import { LogViewer } from "./components/LogViewer";
import { EnvPanel } from "./components/EnvPanel";
import { NewAppModal } from "./components/NewAppModal";
import type { App, Deployment, Theme } from "./types";

export default function Home() {
  const [theme, setTheme] = useState<Theme>("dark");
  const [screen, setScreen] = useState<"landing" | "login">("landing");
  const [ready, setReady] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [apiIssue, setApiIssue] = useState("");

  const [showNewApp, setShowNewApp] = useState(false);
  const [newAppName, setNewAppName] = useState("");
  const [deployFile, setDeployFile] = useState<File | null>(null);
  const [envName, setEnvName] = useState("");
  const [envValue, setEnvValue] = useState("");

  const [loggingIn, setLoggingIn] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);
  const [creatingApp, setCreatingApp] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [rollingBackID, setRollingBackID] = useState("");
  const [savingEnv, setSavingEnv] = useState(false);
  const [deletingEnvKey, setDeletingEnvKey] = useState("");

  const handleApiIssue = useCallback((message: string) => setApiIssue(message), []);
  const {
    apps,
    selectedName,
    selectedApp,
    deployments,
    envKeys,
    setSelectedName,
    refreshApps,
    refreshApp,
    resetApps,
  } = useApps(authenticated, handleApiIssue);
  const { logs, outputRef, clearLogs } = useLogStream(authenticated, selectedName);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const saved = window.localStorage.getItem("minipaas-theme");
      if (saved === "light" || saved === "dark") setTheme(saved);
      refreshApps()
        .then(() => {
          setAuthenticated(true);
          setApiIssue("");
        })
        .catch((cause: unknown) => {
          setAuthenticated(false);
          if (!(cause instanceof ApiError && cause.status === 401)) {
            setApiIssue(cause instanceof Error ? cause.message : "Não foi possível conectar à API.");
          }
        })
        .finally(() => setReady(true));
    }, 0);
    return () => window.clearTimeout(timer);
  }, [refreshApps]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    window.localStorage.setItem("minipaas-theme", theme);
  }, [theme]);

  const toggleTheme = () => setTheme((current) => current === "dark" ? "light" : "dark");
  const clearFeedback = () => {
    setError("");
    setNotice("");
    setApiIssue("");
  };

  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoggingIn(true);
    setError("");
    const form = new FormData(event.currentTarget);
    try {
      await request("/auth/web-login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username: form.get("username"), password: form.get("password") }),
      });
      await refreshApps();
      setAuthenticated(true);
      setApiIssue("");
    } catch (cause) {
      if (cause instanceof ApiError && cause.status !== 401) setApiIssue(cause.message);
      setError(cause instanceof Error ? cause.message : "Não foi possível entrar.");
    } finally {
      setLoggingIn(false);
    }
  }

  async function logout() {
    setLoggingOut(true);
    try {
      await request("/auth/logout", { method: "POST" });
      setAuthenticated(false);
      resetApps();
      clearLogs();
      clearFeedback();
      setScreen("landing");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível encerrar a sessão.");
    } finally {
      setLoggingOut(false);
    }
  }

  async function createApp(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!newAppName.trim()) return;
    setCreatingApp(true);
    setError("");
    try {
      const app = await request<App>("/apps", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: newAppName.trim() }),
      });
      setShowNewApp(false);
      setNewAppName("");
      setSelectedName(app.name);
      await refreshApps();
      setNotice(`Aplicação ${app.name} criada.`);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível criar a aplicação.");
    } finally {
      setCreatingApp(false);
    }
  }

  async function deploy(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedApp || !deployFile) return;
    setDeploying(true);
    setError("");
    try {
      const data = new FormData();
      data.append("source", deployFile);
      await request<Deployment>(`/apps/${encodeURIComponent(selectedApp.name)}/deployments`, { method: "POST", body: data });
      setDeployFile(null);
      setNotice("Deploy enviado. Acompanhe a construção nos logs.");
      await refreshApp(selectedApp.name);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível iniciar o deploy.");
    } finally {
      setDeploying(false);
    }
  }

  async function rollback(deploymentID: string) {
    if (!selectedApp) return;
    setRollingBackID(deploymentID);
    setError("");
    try {
      await request(`/apps/${encodeURIComponent(selectedApp.name)}/rollback`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ deployment_id: deploymentID, triggered_by: "dashboard" }),
      });
      setNotice("Rollback iniciado.");
      await refreshApp(selectedApp.name);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível fazer rollback.");
    } finally {
      setRollingBackID("");
    }
  }

  async function saveEnv(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedApp || !envName.trim()) return;
    setSavingEnv(true);
    setError("");
    try {
      await request(`/apps/${encodeURIComponent(selectedApp.name)}/env/${encodeURIComponent(envName.trim())}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: envValue }),
      });
      setEnvName("");
      setEnvValue("");
      setNotice("Variável salva. Ela será aplicada no próximo deploy.");
      await refreshApp(selectedApp.name);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível salvar a variável.");
    } finally {
      setSavingEnv(false);
    }
  }

  async function deleteEnv(key: string) {
    if (!selectedApp) return;
    setDeletingEnvKey(key);
    setError("");
    try {
      await request(`/apps/${encodeURIComponent(selectedApp.name)}/env/${encodeURIComponent(key)}`, { method: "DELETE" });
      setNotice(`Variável ${key} removida.`);
      await refreshApp(selectedApp.name);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível remover a variável.");
    } finally {
      setDeletingEnvKey("");
    }
  }

  if (!ready) return <main className="loading-screen">Iniciando o painel…</main>;
  if (!authenticated && screen === "landing") {
    return <Landing theme={theme} onThemeChange={toggleTheme} onAccess={() => setScreen("login")} apiIssue={apiIssue} />;
  }
  if (!authenticated) {
    return <LoginScreen theme={theme} onThemeChange={toggleTheme} onBack={() => setScreen("landing")} onLogin={login} error={error} apiIssue={apiIssue} busy={loggingIn} />;
  }

  return (
    <main className="app-shell" data-theme={theme}>
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">M</span><span>MiniPaaS</span></div>
        <nav aria-label="Navegação principal">
          <a className="nav-item active" href="#overview"><span>▦</span>Visão geral</a>
          <a className="nav-item" href="#deployments"><span>◫</span>Deploys</a>
          <a className="nav-item" href="#environment"><span>⌘</span>Ambiente</a>
        </nav>
        <div className="sidebar-footer"><span className="status-dot healthy" /> API conectada</div>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div><p className="eyebrow">PLATAFORMA</p><h1>Aplicações</h1></div>
          <div className="topbar-actions">
            <button className="theme-button" onClick={logout} disabled={loggingOut}>{loggingOut ? "Saindo…" : "Sair"}</button>
            <button className="theme-button" onClick={toggleTheme}>{theme === "dark" ? "☼ Claro" : "◐ Escuro"}</button>
            <button className="button primary" onClick={() => setShowNewApp(true)}>＋ Nova aplicação</button>
          </div>
        </header>

        {(apiIssue || error || notice) && (
          <div className={`feedback ${apiIssue || error ? "error" : "success"}`} role="status">
            {apiIssue || error || notice}<button onClick={clearFeedback} aria-label="Fechar aviso">×</button>
          </div>
        )}

        <section id="overview" className="overview-grid">
          <AppList apps={apps} selectedName={selectedName} onSelect={(name) => { setSelectedName(name); setError(""); }} />
          <DeployPanel app={selectedApp} deployments={deployments} deployFile={deployFile} deploying={deploying} onFileChange={setDeployFile} onDeploy={deploy} onCreate={() => setShowNewApp(true)} />
        </section>

        {selectedApp && (
          <section className="operations-grid">
            <DeploymentList deployments={deployments} rollingBackID={rollingBackID} onRollback={rollback} />
            <LogViewer logs={logs} outputRef={outputRef} />
            <EnvPanel envKeys={envKeys} envName={envName} envValue={envValue} saving={savingEnv} deletingKey={deletingEnvKey} onNameChange={setEnvName} onValueChange={setEnvValue} onSave={saveEnv} onDelete={deleteEnv} />
          </section>
        )}
      </section>

      {showNewApp && <NewAppModal name={newAppName} creating={creatingApp} onNameChange={setNewAppName} onClose={() => setShowNewApp(false)} onSubmit={createApp} />}
    </main>
  );
}
