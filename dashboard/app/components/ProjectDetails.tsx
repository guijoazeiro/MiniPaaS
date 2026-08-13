"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { ApiError, formatTime, request, stateLabel } from "../lib/api";
import { useLogStream } from "../hooks/useLogStream";
import type { App, Deployment, EnvKey, GitHubInstallation, GitHubRepository, GitSource } from "../types";
import { DeployPanel } from "./DeployPanel";
import { DeploymentList } from "./DeploymentList";
import { DeploymentLogsPanel } from "./DeploymentLogsPanel";
import { EnvPanel } from "./EnvPanel";
import { GitDeployPanel } from "./GitDeployPanel";
import { LogViewer } from "./LogViewer";
import { useDashboard } from "./DashboardShell";

type Tab = "overview" | "deployments" | "logs" | "settings";
const tabs: { id: Tab; label: string }[] = [
  { id: "overview", label: "Visão geral" },
  { id: "deployments", label: "Deployments" },
  { id: "logs", label: "Logs" },
  { id: "settings", label: "Configurações" },
];

export function ProjectDetails({ name }: { name: string }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedTab = searchParams.get("tab") as Tab | null;
  const tab: Tab = tabs.some((item) => item.id === requestedTab) ? requestedTab as Tab : "overview";
  const { refreshApps, setFeedback } = useDashboard();
  const [app, setApp] = useState<App | null>(null);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [envKeys, setEnvKeys] = useState<EnvKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [deployFile, setDeployFile] = useState<File | null>(null);
  const [deploying, setDeploying] = useState(false);
  const [rollingBackID, setRollingBackID] = useState("");
  const [stoppingApp, setStoppingApp] = useState(false);
  const [confirmingStop, setConfirmingStop] = useState(false);
  const [envName, setEnvName] = useState("");
  const [envValue, setEnvValue] = useState("");
  const [savingEnv, setSavingEnv] = useState(false);
  const [deletingEnvKey, setDeletingEnvKey] = useState("");
  const [gitSource, setGitSource] = useState<GitSource | null>(null);
  const [gitMode, setGitMode] = useState<"public" | "github_app">("public");
  const [gitRepository, setGitRepository] = useState("");
  const [gitBranch, setGitBranch] = useState("main");
  const [gitBuildContext, setGitBuildContext] = useState(".");
  const [gitDockerfile, setGitDockerfile] = useState("Dockerfile");
  const [savingGit, setSavingGit] = useState(false);
  const [deployingGit, setDeployingGit] = useState(false);
  const [disconnectingGit, setDisconnectingGit] = useState(false);
  const [githubEnabled, setGitHubEnabled] = useState(false);
  const [githubWebhooksEnabled, setGitHubWebhooksEnabled] = useState(false);
  const [githubLoading, setGitHubLoading] = useState(false);
  const [githubInstallations, setGitHubInstallations] = useState<GitHubInstallation[]>([]);
  const [githubRepositories, setGitHubRepositories] = useState<GitHubRepository[]>([]);
  const [githubInstallationID, setGitHubInstallationID] = useState("");
  const [githubRepositoryID, setGitHubRepositoryID] = useState("");
  const [togglingAutoDeploy, setTogglingAutoDeploy] = useState(false);
  const logStream = useLogStream(true, tab === "logs" ? name : "");

  const refreshProject = useCallback(async () => {
    const encodedName = encodeURIComponent(name);
    const [nextApp, nextDeployments, nextEnv] = await Promise.all([
      request<App>(`/apps/${encodedName}`),
      request<Deployment[]>(`/apps/${encodedName}/deployments`),
      request<EnvKey[]>(`/apps/${encodedName}/env`),
    ]);
    setApp(nextApp);
    setDeployments(nextDeployments);
    setEnvKeys(nextEnv);
    return nextApp;
  }, [name]);

  useEffect(() => {
    let active = true;
    const initialTimer = window.setTimeout(() => {
      refreshProject()
        .catch((cause: unknown) => {
          if (!active) return;
          if (cause instanceof ApiError && cause.status === 404) router.replace("/dashboard/projects");
          else setFeedback(cause instanceof Error ? cause.message : "Não foi possível carregar o projeto.", "error");
        })
        .finally(() => { if (active) setLoading(false); });
    }, 0);
    const timer = window.setInterval(() => refreshProject().catch(() => undefined), 5000);
    return () => { active = false; window.clearTimeout(initialTimer); window.clearInterval(timer); };
  }, [refreshProject, router, setFeedback]);

  useEffect(() => {
    let active = true;
    request<GitSource>(`/apps/${encodeURIComponent(name)}/source/git`)
      .then((source) => {
        if (!active) return;
        setGitSource(source);
        setGitRepository(source.repository);
        setGitBranch(source.branch);
        setGitBuildContext(source.build_context);
        setGitDockerfile(source.dockerfile_path);
        setGitMode(source.access_mode || "public");
        setGitHubInstallationID(source.github_installation_id ? String(source.github_installation_id) : "");
        setGitHubRepositoryID(source.github_repository_id ? String(source.github_repository_id) : "");
      })
      .catch((cause: unknown) => {
        if (!active) return;
        if (cause instanceof ApiError && cause.status === 404) {
          setGitSource(null);
          return;
        }
        setFeedback(cause instanceof Error ? cause.message : "Não foi possível carregar a origem Git.", "error");
      });
    return () => { active = false; };
  }, [name, setFeedback]);

  const loadGitHubInstallations = useCallback(async () => {
    setGitHubLoading(true);
    try {
      const status = await request<{ enabled: boolean; webhooks_enabled: boolean }>("/integrations/github/status");
      setGitHubEnabled(status.enabled);
      setGitHubWebhooksEnabled(status.webhooks_enabled);
      if (!status.enabled) {
        setGitHubInstallations([]);
        setGitHubRepositories([]);
        return;
      }
      const installations = await request<GitHubInstallation[]>("/integrations/github/installations");
      setGitHubInstallations(installations);
      setGitHubInstallationID((current) => current || (installations[0] ? String(installations[0].installation_id) : ""));
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível carregar a integração com o GitHub.", "error");
    } finally {
      setGitHubLoading(false);
    }
  }, [setFeedback]);

  useEffect(() => {
    const timer = window.setTimeout(() => { void loadGitHubInstallations(); }, 0);
    return () => window.clearTimeout(timer);
  }, [loadGitHubInstallations]);

  useEffect(() => {
    if (!githubEnabled || !githubInstallationID) return;
    let active = true;
    const timer = window.setTimeout(() => {
      setGitHubLoading(true);
      request<GitHubRepository[]>(`/integrations/github/installations/${encodeURIComponent(githubInstallationID)}/repositories`)
        .then((repositories) => { if (active) setGitHubRepositories(repositories); })
        .catch((cause: unknown) => { if (active) setFeedback(cause instanceof Error ? cause.message : "Não foi possível carregar os repositórios.", "error"); })
        .finally(() => { if (active) setGitHubLoading(false); });
    }, 0);
    return () => { active = false; window.clearTimeout(timer); };
  }, [githubEnabled, githubInstallationID, setFeedback]);

  useEffect(() => {
    if (searchParams.get("github") !== "connected") return;
    const timer = window.setTimeout(() => {
      setGitMode("github_app");
      setFeedback("GitHub App conectado. Selecione o repositório privado.");
      void loadGitHubInstallations();
      router.replace(`/dashboard/projects/${encodeURIComponent(name)}?tab=settings`);
    }, 0);
    return () => window.clearTimeout(timer);
  }, [loadGitHubInstallations, name, router, searchParams, setFeedback]);

  function changeTab(nextTab: Tab) {
    router.replace(nextTab === "overview" ? `/dashboard/projects/${encodeURIComponent(name)}` : `/dashboard/projects/${encodeURIComponent(name)}?tab=${nextTab}`);
  }

  function viewDeploymentLogs(deploymentID: string) {
    router.replace(`/dashboard/projects/${encodeURIComponent(name)}?tab=logs&deployment=${encodeURIComponent(deploymentID)}`);
  }

  async function deploy(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!deployFile) return;
    setDeploying(true);
    try {
      const data = new FormData();
      data.append("source", deployFile);
      await request<Deployment>(`/apps/${encodeURIComponent(name)}/deployments`, { method: "POST", body: data });
      setDeployFile(null);
      setFeedback("Deploy enviado. Acompanhe a construção nos logs.");
      await refreshProject();
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível iniciar o deploy.", "error");
    } finally {
      setDeploying(false);
    }
  }

  async function rollback(deploymentID: string) {
    setRollingBackID(deploymentID);
    try {
      await request(`/apps/${encodeURIComponent(name)}/rollback`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ deployment_id: deploymentID, triggered_by: "dashboard" }) });
      setFeedback("Rollback iniciado.");
      await Promise.all([refreshProject(), refreshApps()]);
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível fazer rollback.", "error");
    } finally {
      setRollingBackID("");
    }
  }

  async function stopApp() {
    if (!confirmingStop) { setConfirmingStop(true); return; }
    setStoppingApp(true);
    try {
      await request(`/apps/${encodeURIComponent(name)}/stop`, { method: "POST" });
      setConfirmingStop(false);
      setFeedback(`Aplicação ${name} parada.`);
      await Promise.all([refreshProject(), refreshApps()]);
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível parar a aplicação.", "error");
    } finally {
      setStoppingApp(false);
    }
  }

  async function saveGitSource(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (gitMode === "public" && !gitRepository.trim()) return;
    if (gitMode === "github_app" && (!githubInstallationID || !githubRepositoryID)) return;
    setSavingGit(true);
    try {
      const path = gitMode === "public" ? "source/git" : "source/github-app";
      const body = gitMode === "public"
        ? { repository: gitRepository.trim(), branch: gitBranch, build_context: gitBuildContext, dockerfile_path: gitDockerfile }
        : { installation_id: Number(githubInstallationID), repository_id: Number(githubRepositoryID), branch: gitBranch, build_context: gitBuildContext, dockerfile_path: gitDockerfile };
      const source = await request<GitSource>(`/apps/${encodeURIComponent(name)}/${path}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
      setGitSource(source);
      setGitRepository(source.repository);
      setGitBranch(source.branch);
      setFeedback(`Repositório ${source.repository} conectado.`);
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível conectar o repositório.", "error");
    } finally {
      setSavingGit(false);
    }
  }

  async function installGitHubApp() {
    setGitHubLoading(true);
    try {
      const response = await request<{ url: string }>(`/integrations/github/install-url?app=${encodeURIComponent(name)}`);
      window.location.assign(response.url);
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível iniciar a instalação do GitHub App.", "error");
      setGitHubLoading(false);
    }
  }

  function selectPrivateRepository(value: string) {
    setGitHubRepositoryID(value);
    const repository = githubRepositories.find((item) => String(item.id) === value);
    if (!repository) return;
    setGitRepository(repository.full_name);
    setGitBranch(repository.default_branch || "main");
  }

  function selectGitHubInstallation(value: string) {
    setGitHubInstallationID(value);
    setGitHubRepositoryID("");
    setGitHubRepositories([]);
  }

  async function toggleAutoDeploy() {
    if (!gitSource) return;
    setTogglingAutoDeploy(true);
    try {
      const source = await request<GitSource>(`/apps/${encodeURIComponent(name)}/source/git/auto-deploy`, {
        method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ enabled: !gitSource.auto_deploy }),
      });
      setGitSource(source);
      setFeedback(source.auto_deploy ? `Deploy automático habilitado para ${source.branch}.` : "Deploy automático desabilitado.");
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível alterar o deploy automático.", "error");
    } finally {
      setTogglingAutoDeploy(false);
    }
  }

  async function deployGit() {
    if (!gitSource) return;
    setDeployingGit(true);
    try {
      await request<Deployment>(`/apps/${encodeURIComponent(name)}/deployments/git`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ branch: gitBranch }) });
      setFeedback("Deploy Git iniciado. Acompanhe o status e os logs.");
      await refreshProject();
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível iniciar o deploy Git.", "error");
    } finally {
      setDeployingGit(false);
    }
  }

  async function disconnectGit() {
    if (!gitSource) return;
    setDisconnectingGit(true);
    try {
      await request(`/apps/${encodeURIComponent(name)}/source/git`, { method: "DELETE" });
      setGitSource(null); setGitMode("public"); setGitRepository(""); setGitBranch("main"); setGitBuildContext("."); setGitDockerfile("Dockerfile"); setGitHubRepositoryID("");
      setFeedback("Repositório desconectado.");
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível desconectar o repositório.", "error");
    } finally {
      setDisconnectingGit(false);
    }
  }

  async function saveEnv(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!envName.trim()) return;
    setSavingEnv(true);
    try {
      await request(`/apps/${encodeURIComponent(name)}/env/${encodeURIComponent(envName.trim())}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ value: envValue }) });
      setEnvName(""); setEnvValue(""); setFeedback("Variável salva. Ela será aplicada no próximo deploy.");
      await refreshProject();
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível salvar a variável.", "error");
    } finally {
      setSavingEnv(false);
    }
  }

  async function deleteEnv(key: string) {
    setDeletingEnvKey(key);
    try {
      await request(`/apps/${encodeURIComponent(name)}/env/${encodeURIComponent(key)}`, { method: "DELETE" });
      setFeedback(`Variável ${key} removida.`);
      await refreshProject();
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível remover a variável.", "error");
    } finally {
      setDeletingEnvKey("");
    }
  }

  if (loading || !app) return <div className="list-placeholder">Carregando projeto…</div>;

  return (
    <>
      <header className="project-heading">
        <div><Link className="back-link" href="/dashboard/projects">Voltar para projetos</Link><div className="project-title-line"><h1>{app.name}</h1><span className={`status-pill ${app.status}`}><i />{stateLabel(app.status)}</span></div><p className="page-description">Atualizado em {formatTime(app.updated_at)}</p></div>
        <div className="project-heading-actions">{app.public_url && <a className="button secondary button-link" href={app.public_url} target="_blank" rel="noreferrer">Abrir aplicação</a>}<Link className="button secondary button-link" href={`/dashboard/logs?app=${encodeURIComponent(name)}`}>Logs em tela cheia</Link></div>
      </header>

      <nav className="project-tabs" aria-label="Seções do projeto">{tabs.map((item) => <button key={item.id} className={tab === item.id ? "active" : ""} onClick={() => changeTab(item.id)}>{item.label}</button>)}</nav>

      {tab === "overview" && <div className="project-section-stack"><DeployPanel app={app} deployments={deployments} deployFile={deployFile} deploying={deploying} stopping={stoppingApp} confirmingStop={confirmingStop} onFileChange={setDeployFile} onDeploy={deploy} onCreate={() => undefined} onRequestStop={stopApp} onCancelStop={() => setConfirmingStop(false)} /><section className="panel project-summary"><div className="section-heading"><div><p className="eyebrow">ÚLTIMA ATIVIDADE</p><h2>Resumo operacional</h2></div></div><div className="summary-grid"><div><span>Origem</span><strong>{gitSource ? "GitHub" : "Upload manual"}</strong><small>{gitSource?.repository || "Nenhum repositório conectado"}</small></div><div><span>Deployments</span><strong>{deployments.length}</strong><small>{deployments[0] ? `Último: ${stateLabel(deployments[0].status)}` : "Nenhum release"}</small></div><div><span>Variáveis</span><strong>{envKeys.length}</strong><small>nomes configurados</small></div></div></section></div>}
      {tab === "deployments" && <DeploymentList deployments={deployments} rollingBackID={rollingBackID} onRollback={rollback} onViewLogs={viewDeploymentLogs} />}
      {tab === "logs" && <div className="project-section-stack">{searchParams.get("deployment") && <DeploymentLogsPanel appName={name} deploymentID={searchParams.get("deployment")!} />}<LogViewer logs={logStream.logs} outputRef={logStream.outputRef} following={logStream.following} connection={logStream.connection} dedicated onScroll={logStream.handleScroll} onResume={logStream.resumeFollowing} onClear={logStream.clearLogs} /></div>}
      {tab === "settings" && <div className="project-section-stack"><GitDeployPanel source={gitSource} mode={gitMode} repository={gitRepository} branch={gitBranch} buildContext={gitBuildContext} dockerfilePath={gitDockerfile} githubEnabled={githubEnabled} githubLoading={githubLoading} webhooksEnabled={githubWebhooksEnabled} installations={githubInstallations} repositories={githubRepositories} selectedInstallationID={githubInstallationID} selectedRepositoryID={githubRepositoryID} saving={savingGit} deploying={deployingGit} disconnecting={disconnectingGit} togglingAutoDeploy={togglingAutoDeploy} onModeChange={setGitMode} onRepositoryChange={setGitRepository} onBranchChange={setGitBranch} onBuildContextChange={setGitBuildContext} onDockerfilePathChange={setGitDockerfile} onInstallationChange={selectGitHubInstallation} onPrivateRepositoryChange={selectPrivateRepository} onInstallGitHubApp={installGitHubApp} onToggleAutoDeploy={toggleAutoDeploy} onSave={saveGitSource} onDeploy={deployGit} onDisconnect={disconnectGit} /><EnvPanel envKeys={envKeys} envName={envName} envValue={envValue} saving={savingEnv} deletingKey={deletingEnvKey} onNameChange={setEnvName} onValueChange={setEnvValue} onSave={saveEnv} onDelete={deleteEnv} /></div>}
    </>
  );
}
