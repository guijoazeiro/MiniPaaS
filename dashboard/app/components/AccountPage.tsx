"use client";

import { useEffect, useState, type FormEvent } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { formatTime, request } from "../lib/api";
import type { GitHubInstallation, User } from "../types";
import { GitHubIcon } from "./GitHubIcon";
import { useDashboard } from "./DashboardShell";

export function AccountPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { setFeedback } = useDashboard();
  const [user, setUser] = useState<User | null>(null);
  const [username, setUsername] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [loading, setLoading] = useState(true);
  const [savingUsername, setSavingUsername] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);
  const [error, setError] = useState("");
  const [githubEnabled, setGitHubEnabled] = useState(false);
  const [githubInstallations, setGitHubInstallations] = useState<GitHubInstallation[]>([]);
  const [githubLoading, setGitHubLoading] = useState(true);
  const [githubInstalling, setGitHubInstalling] = useState(false);

  async function loadGitHubInstallations() {
    setGitHubLoading(true);
    try {
      const status = await request<{ enabled: boolean }>("/integrations/github/status");
      setGitHubEnabled(status.enabled);
      if (!status.enabled) {
        setGitHubInstallations([]);
        return;
      }
      setGitHubInstallations(await request<GitHubInstallation[]>("/integrations/github/installations"));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível carregar as conexões do GitHub.");
    } finally {
      setGitHubLoading(false);
    }
  }

  useEffect(() => {
    let active = true;
    request<User>("/me")
      .then((value) => {
        if (!active) return;
        setUser(value);
        setUsername(value.username);
      })
      .catch((cause: unknown) => {
        if (active) setError(cause instanceof Error ? cause.message : "Não foi possível carregar a conta.");
      })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => { void loadGitHubInstallations(); }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (searchParams.get("github") !== "connected") return;
    setFeedback("GitHub App conectado à sua conta MiniPaaS.");
    const timer = window.setTimeout(() => { void loadGitHubInstallations(); }, 0);
    router.replace("/dashboard/account");
    return () => window.clearTimeout(timer);
  }, [router, searchParams, setFeedback]);

  async function updateUsername(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSavingUsername(true);
    setError("");
    try {
      const updated = await request<User>("/me", { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ username: username.trim() }) });
      setUser(updated);
      setUsername(updated.username);
      setFeedback("Usuário atualizado.");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível atualizar o usuário.");
    } finally {
      setSavingUsername(false);
    }
  }

  async function updatePassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (newPassword !== confirmation) {
      setError("As senhas não coincidem.");
      return;
    }
    setSavingPassword(true);
    setError("");
    try {
      await request("/me/password", { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmation("");
      setFeedback("Senha atualizada.");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível atualizar a senha.");
    } finally {
      setSavingPassword(false);
    }
  }

  async function installGitHubApp() {
    setGitHubInstalling(true);
    try {
      const response = await request<{ url: string }>("/integrations/github/account-install-url");
      window.location.assign(response.url);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Não foi possível iniciar a instalação do GitHub App.");
      setGitHubInstalling(false);
    }
  }

  if (loading) return <div className="list-placeholder">Carregando conta…</div>;
  if (!user) return <div className="list-placeholder">{error || "Conta indisponível."}</div>;

  return (
    <div className="account-page">
      <header className="page-heading"><div><p className="eyebrow">CONFIGURAÇÕES</p><h1>Minha conta</h1><p className="page-description">Atualize seus dados básicos e mantenha o acesso protegido.</p></div></header>
      {error && <div className="feedback error" role="alert"><span>{error}</span><button onClick={() => setError("")} aria-label="Fechar aviso">Fechar</button></div>}
      <div className="account-grid">
        <section className="panel account-card">
          <div className="section-heading"><div><p className="eyebrow">PERFIL</p><h2>Dados da conta</h2></div></div>
          <div className="account-meta"><span>ID</span><code>{user.id}</code><span>Criada em</span><strong>{formatTime(user.created_at)}</strong></div>
          <form className="form-stack" onSubmit={updateUsername}>
            <label>Usuário<input value={username} onChange={(event) => setUsername(event.target.value)} minLength={3} maxLength={64} required /></label>
            <button className="button primary" disabled={savingUsername || username.trim() === user.username}>{savingUsername ? "Salvando…" : "Salvar usuário"}</button>
          </form>
        </section>
        <section className="panel account-card">
          <div className="section-heading"><div><p className="eyebrow">SEGURANÇA</p><h2>Alterar senha</h2></div></div>
          <form className="form-stack" onSubmit={updatePassword}>
            <label>Senha atual<input type="password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} autoComplete="current-password" required /></label>
            <label>Nova senha<input type="password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} minLength={8} autoComplete="new-password" required /></label>
            <label>Confirmar nova senha<input type="password" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} minLength={8} autoComplete="new-password" required /></label>
            <button className="button primary" disabled={savingPassword}>{savingPassword ? "Salvando…" : "Atualizar senha"}</button>
          </form>
        </section>
      </div>
      <section className="panel account-card">
        <div className="section-heading"><div><p className="eyebrow">INTEGRAÇÕES</p><h2>Contas do GitHub</h2></div><GitHubIcon size={22} /></div>
        <p className="panel-description">Conecte uma conta ou organização para acessar repositórios privados durante os deploys.</p>
        {githubLoading ? <p className="panel-description">Carregando instalações…</p> : !githubEnabled ? <p className="panel-description">O GitHub App ainda não está configurado nesta instância.</p> : (
          <>
            {githubInstallations.length > 0 ? <div className="github-installations" aria-label="Contas GitHub conectadas">{githubInstallations.map((installation) => <div className="github-installation" key={installation.installation_id}><strong>{installation.account_login}</strong><span>{installation.account_type} · acesso {installation.repository_selection}</span></div>)}</div> : <p className="panel-description">Nenhuma conta GitHub conectada.</p>}
            <button className="button secondary github-account-action" type="button" onClick={installGitHubApp} disabled={githubInstalling}><GitHubIcon size={16} /> {githubInstalling ? "Abrindo GitHub…" : githubInstallations.length ? "Conectar outra conta" : "Conectar GitHub"}</button>
          </>
        )}
      </section>
      <p className="panel-description account-note">A MiniPaaS não armazena senhas em texto puro. A sessão é renovada após alterações de credenciais.</p>
    </div>
  );
}
