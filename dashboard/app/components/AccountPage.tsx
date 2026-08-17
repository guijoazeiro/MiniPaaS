"use client";

import { useEffect, useState, type FormEvent } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { formatTime, request } from "../lib/api";
import type { APIToken, APITokenCreated, GitHubInstallation, User } from "../types";
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
  const [apiTokens, setAPITokens] = useState<APIToken[]>([]);
  const [tokensLoading, setTokensLoading] = useState(true);
  const [tokenError, setTokenError] = useState("");
  const [showCreateToken, setShowCreateToken] = useState(false);
  const [tokenName, setTokenName] = useState("");
  const [tokenScopes, setTokenScopes] = useState<string[]>(["read"]);
  const [tokenExpiresAt, setTokenExpiresAt] = useState("");
  const [tokenCreating, setTokenCreating] = useState(false);
  const [tokenToRevoke, setTokenToRevoke] = useState<APIToken | null>(null);
  const [revokingTokenId, setRevokingTokenId] = useState("");
  const [createdToken, setCreatedToken] = useState<APITokenCreated | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);

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

  async function loadAPITokens() {
    setTokensLoading(true);
    try {
      setAPITokens(await request<APIToken[]>("/me/tokens"));
      setTokenError("");
    } catch (cause) {
      setTokenError(cause instanceof Error ? cause.message : "Não foi possível carregar os tokens.");
    } finally {
      setTokensLoading(false);
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
    const timer = window.setTimeout(() => { void loadAPITokens(); }, 0);
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

  function toggleTokenScope(scope: string) {
    setTokenScopes((current) => current.includes(scope) ? current.filter((item) => item !== scope) : [...current, scope]);
  }

  function closeCreateToken(force = false) {
    if (tokenCreating && !force) return;
    setShowCreateToken(false);
    setTokenName("");
    setTokenScopes(["read"]);
    setTokenExpiresAt("");
  }

  async function createAPIToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!tokenName.trim() || tokenScopes.length === 0) return;
    setTokenCreating(true);
    setTokenError("");
    try {
      const expiresAt = tokenExpiresAt ? new Date(`${tokenExpiresAt}T23:59:59Z`).toISOString() : null;
      const created = await request<APITokenCreated>("/me/tokens", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: tokenName.trim(), scopes: tokenScopes, expires_at: expiresAt }),
      });
      await loadAPITokens();
      closeCreateToken(true);
      setCreatedToken(created);
      setTokenCopied(false);
      setFeedback("Token criado. Copie o segredo agora, ele não será exibido novamente.");
    } catch (cause) {
      setTokenError(cause instanceof Error ? cause.message : "Não foi possível criar o token.");
    } finally {
      setTokenCreating(false);
    }
  }

  async function revokeAPIToken() {
    if (!tokenToRevoke) return;
    setRevokingTokenId(tokenToRevoke.id);
    setTokenError("");
    try {
      await request(`/me/tokens/${encodeURIComponent(tokenToRevoke.id)}`, { method: "DELETE" });
      setTokenToRevoke(null);
      await loadAPITokens();
      setFeedback("Token revogado.");
    } catch (cause) {
      setTokenError(cause instanceof Error ? cause.message : "Não foi possível revogar o token.");
    } finally {
      setRevokingTokenId("");
    }
  }

  async function copyCreatedToken() {
    if (!createdToken) return;
    try {
      await navigator.clipboard.writeText(createdToken.token);
      setTokenCopied(true);
    } catch {
      setTokenError("Não foi possível copiar o token. Copie-o manualmente enquanto esta janela estiver aberta.");
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
      <section className="panel account-card api-tokens-card">
        <div className="section-heading"><div><p className="eyebrow">AUTOMAÇÃO</p><h2>API tokens</h2></div><button className="button primary compact-action" type="button" onClick={() => setShowCreateToken(true)}>Criar token</button></div>
        <p className="panel-description">Use tokens pessoais para autenticar automações e pipelines de CI/CD sem compartilhar sua sessão.</p>
        {tokenError && <div className="feedback error api-token-feedback" role="alert"><span>{tokenError}</span><button type="button" onClick={() => setTokenError("")} aria-label="Fechar aviso">×</button></div>}
        {tokensLoading ? <p className="panel-description">Carregando tokens…</p> : apiTokens.length === 0 ? <p className="panel-description">Nenhum token criado.</p> : (
          <div className="api-token-list" aria-label="API tokens">
            {apiTokens.map((token) => {
              const revoked = Boolean(token.revoked_at);
              return <div className={`api-token-row ${revoked ? "revoked" : ""}`} key={token.id}>
                <div className="api-token-identity"><strong>{token.name}</strong><code>{token.token_prefix}…</code></div>
                <div className="api-token-scopes">{token.scopes.map((scope) => <span key={scope}>{scope}</span>)}</div>
                <div className="api-token-details"><span>Criado {formatTime(token.created_at)}</span><span>{token.expires_at ? `Expira ${formatTime(token.expires_at)}` : "Sem expiração"}</span><span>{token.last_used_at ? `Usado ${formatTime(token.last_used_at)}` : "Nunca usado"}</span></div>
                {revoked ? <span className="api-token-status">Revogado {formatTime(token.revoked_at)}</span> : <button className="text-button danger" type="button" onClick={() => setTokenToRevoke(token)} disabled={Boolean(revokingTokenId)}>Revogar</button>}
              </div>;
            })}
          </div>
        )}
      </section>
      <p className="panel-description account-note">A MiniPaaS não armazena senhas em texto puro. A sessão é renovada após alterações de credenciais.</p>
      {showCreateToken && <div className="modal-backdrop" role="presentation"><section className="modal api-token-modal" role="dialog" aria-modal="true" aria-labelledby="create-api-token-title">
        <button className="modal-close" type="button" onClick={closeCreateToken} aria-label="Fechar criação de token">×</button>
        <p className="eyebrow">AUTOMAÇÃO</p><h2 id="create-api-token-title">Criar API token</h2>
        <p className="panel-description">Escolha um nome, as permissões necessárias e, opcionalmente, uma data de expiração.</p>
        <form className="form-stack" onSubmit={createAPIToken}>
          <label>Nome do token<input value={tokenName} onChange={(event) => setTokenName(event.target.value)} maxLength={64} placeholder="GitHub Actions" required autoFocus /></label>
          <fieldset className="api-token-scopes-field"><legend>Scopes</legend>{[["read", "Consultar projetos, deployments e logs"], ["deploy", "Fazer deploy, retry e cancelamento"], ["manage", "Criar, parar e configurar aplicações"]].map(([scope, description]) => <label key={scope} className="api-token-scope-option"><input type="checkbox" checked={tokenScopes.includes(scope)} onChange={() => toggleTokenScope(scope)} /><span><strong>{scope}</strong><small>{description}</small></span></label>)}</fieldset>
          <label>Expiração (opcional)<input type="date" value={tokenExpiresAt} onChange={(event) => setTokenExpiresAt(event.target.value)} min={new Date().toISOString().slice(0, 10)} /></label>
          <div className="modal-actions"><button className="button secondary" type="button" onClick={closeCreateToken}>Cancelar</button><button className="button primary" type="submit" disabled={tokenCreating || !tokenName.trim() || tokenScopes.length === 0}>{tokenCreating ? "Criando…" : "Criar token"}</button></div>
        </form>
      </section></div>}
      {tokenToRevoke && <div className="modal-backdrop" role="presentation"><section className="modal api-token-modal" role="dialog" aria-modal="true" aria-labelledby="revoke-api-token-title">
        <button className="modal-close" type="button" onClick={() => setTokenToRevoke(null)} aria-label="Fechar confirmação">×</button>
        <p className="eyebrow">AÇÃO DESTRUTIVA</p><h2 id="revoke-api-token-title">Revogar token?</h2>
        <p className="panel-description">O token <strong>{tokenToRevoke.name}</strong> perderá o acesso imediatamente. Essa ação não pode ser desfeita.</p>
        <div className="modal-actions"><button className="button secondary" type="button" onClick={() => setTokenToRevoke(null)}>Cancelar</button><button className="button danger" type="button" onClick={revokeAPIToken} disabled={Boolean(revokingTokenId)}>{revokingTokenId ? "Revogando…" : "Revogar token"}</button></div>
      </section></div>}
      {createdToken && <div className="modal-backdrop" role="presentation"><section className="modal api-token-modal" role="dialog" aria-modal="true" aria-labelledby="created-api-token-title">
        <button className="modal-close" type="button" onClick={() => { setCreatedToken(null); setTokenCopied(false); }} aria-label="Fechar token criado">×</button>
        <p className="eyebrow">TOKEN CRIADO</p><h2 id="created-api-token-title">Copie seu token agora</h2>
        <p className="panel-description">Por segurança, este segredo só será exibido uma vez. Depois de fechar esta janela, não será possível recuperá-lo.</p>
        <code className="api-token-secret">{createdToken.token}</code>
        <div className="modal-actions"><button className="button secondary" type="button" onClick={copyCreatedToken}>{tokenCopied ? "Copiado" : "Copiar token"}</button><button className="button primary" type="button" onClick={() => { setCreatedToken(null); setTokenCopied(false); }}>Concluir</button></div>
      </section></div>}
    </div>
  );
}
