import type { FormEvent } from "react";
import type { GitHubInstallation, GitHubRepository, GitSource } from "../types";

type GitMode = "public" | "github_app";

type Props = {
  source: GitSource | null;
  mode: GitMode;
  repository: string;
  branch: string;
  buildContext: string;
  dockerfilePath: string;
  githubEnabled: boolean;
  githubLoading: boolean;
  webhooksEnabled: boolean;
  installations: GitHubInstallation[];
  repositories: GitHubRepository[];
  selectedInstallationID: string;
  selectedRepositoryID: string;
  saving: boolean;
  deploying: boolean;
  disconnecting: boolean;
  togglingAutoDeploy: boolean;
  onModeChange: (value: GitMode) => void;
  onRepositoryChange: (value: string) => void;
  onBranchChange: (value: string) => void;
  onBuildContextChange: (value: string) => void;
  onDockerfilePathChange: (value: string) => void;
  onInstallationChange: (value: string) => void;
  onPrivateRepositoryChange: (value: string) => void;
  onInstallGitHubApp: () => void;
  onToggleAutoDeploy: () => void;
  onSave: (event: FormEvent<HTMLFormElement>) => void;
  onDeploy: () => void;
  onDisconnect: () => void;
};

export function GitDeployPanel(props: Props) {
  const canSave = props.mode === "public"
    ? Boolean(props.repository.trim())
    : Boolean(props.selectedInstallationID && props.selectedRepositoryID && props.githubEnabled);

  return (
    <section className="panel glass git-panel">
      <div className="section-heading">
        <div><p className="eyebrow">ORIGEM GIT</p><h2>GitHub</h2></div>
        <span className={`connection-state ${props.source ? "connected" : ""}`}>
          {props.source ? `${props.source.private ? "Privado" : "Público"} conectado` : "Não conectado"}
        </span>
      </div>

      <div className="git-mode-switch" role="group" aria-label="Tipo de acesso ao repositório">
        <button type="button" className={props.mode === "public" ? "active" : ""} onClick={() => props.onModeChange("public")}>Repositório público</button>
        <button type="button" className={props.mode === "github_app" ? "active" : ""} onClick={() => props.onModeChange("github_app")}>GitHub App</button>
      </div>

      <form className="git-source-form" onSubmit={props.onSave}>
        {props.mode === "public" ? (
          <label className="git-repository-field">Repositório público
            <input value={props.repository} onChange={(event) => props.onRepositoryChange(event.target.value)} placeholder="owner/repository" required />
          </label>
        ) : (
          <div className="github-app-fields git-repository-field">
            {!props.githubEnabled ? (
              <div className="integration-notice"><strong>GitHub App não configurado</strong><span>Defina as credenciais do GitHub App na API para habilitar repositórios privados.</span></div>
            ) : (
              <>
                <label>Conta ou organização
                  <select value={props.selectedInstallationID} onChange={(event) => props.onInstallationChange(event.target.value)} disabled={props.githubLoading}>
                    <option value="">Selecione uma instalação</option>
                    {props.installations.map((installation) => <option key={installation.installation_id} value={installation.installation_id}>{installation.account_login}</option>)}
                  </select>
                </label>
                <label>Repositório
                  <select value={props.selectedRepositoryID} onChange={(event) => props.onPrivateRepositoryChange(event.target.value)} disabled={!props.selectedInstallationID || props.githubLoading}>
                    <option value="">Selecione um repositório</option>
                    {props.repositories.map((repository) => <option key={repository.id} value={repository.id}>{repository.full_name}{repository.private ? " · privado" : ""}</option>)}
                  </select>
                </label>
                <button type="button" className="button secondary github-install-button" onClick={props.onInstallGitHubApp} disabled={props.githubLoading}>
                  {props.installations.length ? "Alterar acesso no GitHub" : "Instalar GitHub App"}
                </button>
                <p className="git-help github-account-help">As instalações exibidas pertencem à sua conta MiniPaaS. O botão abre o GitHub para autorizar este App na conta ou organização desejada.</p>
              </>
            )}
          </div>
        )}

        <label>Branch
          <input value={props.branch} onChange={(event) => props.onBranchChange(event.target.value)} placeholder="main" />
        </label>
        <label>Contexto de build
          <input value={props.buildContext} onChange={(event) => props.onBuildContextChange(event.target.value)} placeholder="." />
        </label>
        <label>Dockerfile
          <input value={props.dockerfilePath} onChange={(event) => props.onDockerfilePathChange(event.target.value)} placeholder="Dockerfile" />
        </label>
        <div className="git-source-actions">
          <button className="button secondary" disabled={props.saving || !canSave}>{props.saving ? "Salvando…" : props.source ? "Atualizar conexão" : "Conectar repositório"}</button>
          {props.source && <button type="button" className="button primary" disabled={props.deploying} onClick={props.onDeploy}>{props.deploying ? "Iniciando…" : "Deploy da branch"}</button>}
          {props.source && <button type="button" className="text-button danger" disabled={props.disconnecting} onClick={props.onDisconnect}>{props.disconnecting ? "Desconectando…" : "Desconectar"}</button>}
        </div>
      </form>
      {props.source?.access_mode === "github_app" && (
        <div className="auto-deploy-setting">
          <div><strong>Deploy automático</strong><span>{props.webhooksEnabled ? `Criar um release quando houver push em ${props.source.branch}.` : "Configure o webhook secret na API para habilitar esta opção."}</span></div>
          <button type="button" className={`toggle-control ${props.source.auto_deploy ? "active" : ""}`} role="switch" aria-checked={props.source.auto_deploy} disabled={!props.webhooksEnabled || props.togglingAutoDeploy} onClick={props.onToggleAutoDeploy}><i /></button>
        </div>
      )}
      <p className="git-help">O Dockerfile é resolvido dentro do contexto de build. No modo GitHub App, o MiniPaaS usa um token efêmero e nunca salva a credencial do repositório.</p>
    </section>
  );
}
