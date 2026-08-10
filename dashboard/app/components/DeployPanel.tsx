import type { FormEvent } from "react";
import { formatTime, stateLabel } from "../lib/api";
import type { App, Deployment } from "../types";

type Props = {
  app: App | null;
  deployments: Deployment[];
  deployFile: File | null;
  deploying: boolean;
  onFileChange: (file: File | null) => void;
  onDeploy: (event: FormEvent<HTMLFormElement>) => void;
  onCreate: () => void;
};

export function DeployPanel({ app, deployments, deployFile, deploying, onFileChange, onDeploy, onCreate }: Props) {
  if (!app) {
    return <div className="command-panel"><div className="empty-state"><p className="eyebrow">SEM APLICAÇÃO</p><h2>Comece sua plataforma.</h2><p>Crie uma aplicação e envie o primeiro deploy.</p><button className="button primary" onClick={onCreate}>Criar aplicação</button></div></div>;
  }

  return (
    <div className="command-panel">
      <div className="selected-header"><div><p className="eyebrow">APLICAÇÃO ATIVA</p><h2>{app.name}</h2></div><span className={`status-pill ${app.status}`}><i />{stateLabel(app.status)}</span></div>
      <div className="stats-row">
        <div><span>Status do container</span><strong>{app.container_state || "não iniciado"}</strong></div>
        <div><span>Último deploy</span><strong>{deployments[0] ? formatTime(deployments[0].created_at) : "nenhum"}</strong></div>
        <div><span>URL pública</span>{app.public_url ? <a href={app.public_url} target="_blank" rel="noreferrer">Abrir ↗</a> : <strong>indisponível</strong>}</div>
      </div>
      <form className="deploy-box glass" onSubmit={onDeploy}>
        <div><p className="eyebrow">NOVO RELEASE</p><h3>Enviar código fonte</h3><p>Selecione o arquivo .tar do projeto para iniciar a construção.</p></div>
        <label className="file-picker"><input type="file" accept=".tar,application/x-tar" onChange={(event) => onFileChange(event.target.files?.[0] || null)} /><span>{deployFile ? deployFile.name : "Selecionar arquivo .tar"}</span></label>
        <button className="button primary" disabled={!deployFile || deploying}>{deploying ? "Enviando…" : "Fazer deploy"}</button>
      </form>
    </div>
  );
}
