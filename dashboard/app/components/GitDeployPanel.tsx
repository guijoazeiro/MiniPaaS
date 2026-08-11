import type { FormEvent } from "react";
import type { GitSource } from "../types";

type Props = {
  source: GitSource | null;
  repository: string;
  branch: string;
  buildContext: string;
  dockerfilePath: string;
  saving: boolean;
  deploying: boolean;
  disconnecting: boolean;
  onRepositoryChange: (value: string) => void;
  onBranchChange: (value: string) => void;
  onBuildContextChange: (value: string) => void;
  onDockerfilePathChange: (value: string) => void;
  onSave: (event: FormEvent<HTMLFormElement>) => void;
  onDeploy: () => void;
  onDisconnect: () => void;
};

export function GitDeployPanel(props: Props) {
  return (
    <section className="panel glass git-panel">
      <div className="section-heading">
        <div><p className="eyebrow">ORIGEM GIT</p><h2>GitHub</h2></div>
        <span className={`connection-state ${props.source ? "connected" : ""}`}>{props.source ? "Conectado" : "Não conectado"}</span>
      </div>
      <form className="git-source-form" onSubmit={props.onSave}>
        <label className="git-repository-field">Repositório público
          <input value={props.repository} onChange={(event) => props.onRepositoryChange(event.target.value)} placeholder="owner/repository" required />
        </label>
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
          <button className="button secondary" disabled={props.saving || !props.repository.trim()}>{props.saving ? "Salvando…" : props.source ? "Atualizar conexão" : "Conectar repositório"}</button>
          {props.source && <button type="button" className="button primary" disabled={props.deploying} onClick={props.onDeploy}>{props.deploying ? "Iniciando…" : "Deploy da branch"}</button>}
          {props.source && <button type="button" className="text-button danger" disabled={props.disconnecting} onClick={props.onDisconnect}>{props.disconnecting ? "Desconectando…" : "Desconectar"}</button>}
        </div>
      </form>
      <p className="git-help">Somente repositórios públicos do GitHub. O Dockerfile é resolvido dentro do contexto de build configurado.</p>
    </section>
  );
}
