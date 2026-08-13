"use client";

import { useEffect, useState } from "react";
import { request } from "../lib/api";
import type { DeploymentLog } from "../types";

export function DeploymentLogsPanel({ appName, deploymentID }: { appName: string; deploymentID: string }) {
  const [logs, setLogs] = useState<DeploymentLog[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    const load = () => request<DeploymentLog[]>(`/apps/${encodeURIComponent(appName)}/deployments/${encodeURIComponent(deploymentID)}/logs?limit=1000`)
      .then((items) => { if (active) { setLogs(items); setError(""); } })
      .catch((cause: unknown) => { if (active) setError(cause instanceof Error ? cause.message : "Não foi possível carregar os logs do build."); });
    void load();
    const timer = window.setInterval(load, 2000);
    return () => { active = false; window.clearInterval(timer); };
  }, [appName, deploymentID]);
  return <section className="panel deployment-logs-panel"><div className="section-heading"><div><p className="eyebrow">BUILD</p><h2>Logs persistentes</h2></div><code>{deploymentID.slice(0, 8)}</code></div>{error ? <p className="empty-copy">{error}</p> : <div className="deployment-log-output">{logs.length === 0 ? <span className="empty-copy">Aguardando eventos do build…</span> : logs.map((item) => <div className={`deployment-log-line ${item.stream}`} key={item.id}><span>[{item.stage}]</span> {item.message}</div>)}</div>}</section>;
}
