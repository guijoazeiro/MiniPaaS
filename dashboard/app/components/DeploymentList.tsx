import { formatDuration, formatTime, stateLabel } from "../lib/api";
import type { Deployment } from "../types";

type Props = {
  deployments: Deployment[];
  rollingBackID: string;
  onRollback: (deploymentID: string) => void;
  onViewLogs: (deploymentID: string) => void;
};

export function DeploymentList({ deployments, rollingBackID, onRollback, onViewLogs }: Props) {
  return (
    <section id="deployments" className="panel glass">
      <div className="section-heading"><div><p className="eyebrow">HISTÓRICO</p><h2>Deploys recentes</h2></div><span className="count">{deployments.length}</span></div>
      <div className="deployment-list">
        {deployments.length === 0 ? <p className="empty-copy">Nenhum deploy enviado ainda.</p> : deployments.map((deployment) => (
          <article className="deployment-row" key={deployment.id}>
            <span className={`status-dot ${deployment.status}`} />
            <div>
              <strong>{deployment.source_type === "git" && deployment.commit_sha ? deployment.commit_sha.slice(0, 8) : deployment.image_tag || deployment.id.slice(0, 8)}</strong>
              {deployment.repository && <small className="deployment-source">{deployment.repository}@{deployment.branch || "main"}{deployment.commit_author ? ` · ${deployment.commit_author}` : ""}{deployment.trigger_type === "webhook" ? " · automático" : ""}</small>}
              {deployment.commit_message && <small className="commit-message">{deployment.commit_message.split("\n")[0]}</small>}
              <small>{formatTime(deployment.created_at)} · {formatDuration(deployment.duration_ms)}{deployment.port ? ` · porta ${deployment.port}` : ""}</small>
            </div>
            <span className={`status-pill compact ${deployment.status}`}><i />{stateLabel(deployment.status)}</span>
            <button className="text-button" onClick={() => onViewLogs(deployment.id)}>Logs de build</button>
            {["superseded", "rolled_back", "stopped"].includes(deployment.status) && (
              <button className="text-button" onClick={() => onRollback(deployment.id)} disabled={Boolean(rollingBackID)}>{rollingBackID === deployment.id ? "Revertendo…" : deployment.status === "stopped" ? "Reativar" : "Rollback"}</button>
            )}
          </article>
        ))}
      </div>
    </section>
  );
}
