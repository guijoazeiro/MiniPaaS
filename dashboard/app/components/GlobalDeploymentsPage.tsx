"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { formatDuration, formatTime, request, stateLabel } from "../lib/api";
import type { DeploymentPage } from "../types";
import { useDashboard } from "./DashboardShell";

const deploymentStatuses = ["", "running", "failed", "building", "pending", "stopped", "superseded", "rolled_back"];

export function GlobalDeploymentsPage() {
  const { apps, setFeedback } = useDashboard();
  const [appFilter, setAppFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<DeploymentPage>({ items: [], page: 1, per_page: 25, total: 0 });
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    const query = new URLSearchParams({ page: String(page), per_page: "25" });
    if (appFilter) query.set("app", appFilter);
    if (statusFilter) query.set("status", statusFilter);
    try {
      setResult(await request<DeploymentPage>(`/deployments?${query}`));
    } catch (cause) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível carregar os deployments.", "error");
    } finally {
      setLoading(false);
    }
  }, [appFilter, page, setFeedback, statusFilter]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  const totalPages = Math.max(1, Math.ceil(result.total / result.per_page));

  return (
    <>
      <header className="page-heading">
        <div><p className="eyebrow">TODAS AS APLICAÇÕES</p><h1>Deployments</h1><p className="page-description">Acompanhe releases, falhas e alterações em toda a plataforma.</p></div>
      </header>

      <section className="filter-bar glass" aria-label="Filtros de deployments">
        <label>Projeto<select value={appFilter} onChange={(event) => { setAppFilter(event.target.value); setPage(1); }}><option value="">Todos os projetos</option>{apps.map((app) => <option key={app.id} value={app.name}>{app.name}</option>)}</select></label>
        <label>Status<select value={statusFilter} onChange={(event) => { setStatusFilter(event.target.value); setPage(1); }}>{deploymentStatuses.map((status) => <option key={status || "all"} value={status}>{status ? stateLabel(status) : "Todos os status"}</option>)}</select></label>
        <button className="button secondary" onClick={() => void load()} disabled={loading}>{loading ? "Atualizando…" : "Atualizar"}</button>
        <span className="filter-total">{result.total} deployment{result.total === 1 ? "" : "s"}</span>
      </section>

      <section className="panel deployment-directory">
        {loading && result.items.length === 0 ? <div className="list-placeholder">Carregando deployments…</div> : result.items.length === 0 ? <div className="empty-state roomy"><p className="eyebrow">SEM RESULTADOS</p><h2>Nenhum deployment encontrado.</h2><p>Ajuste os filtros ou inicie um novo release em um projeto.</p></div> : (
          <div className="deployment-table">
            <div className="deployment-table-head"><span>Projeto</span><span>Release</span><span>Status</span><span>Origem</span><span>Duração</span><span>Data</span></div>
            {result.items.map((deployment) => (
              <article className="deployment-table-row" key={deployment.id}>
                <Link href={`/dashboard/projects/${encodeURIComponent(deployment.app_name || "")}`}>{deployment.app_name}</Link>
                <span className="deployment-reference"><strong>{deployment.commit_sha?.slice(0, 8) || deployment.id.slice(0, 8)}</strong><small>{deployment.commit_message?.split("\n")[0] || deployment.image_tag}</small></span>
                <span><span className={`status-pill ${deployment.status}`}><i />{stateLabel(deployment.status)}</span></span>
                <span className="deployment-reference"><strong>{deployment.source_type === "git" ? "GitHub" : "Upload"}</strong><small>{deployment.repository ? `${deployment.repository}@${deployment.branch || "main"}` : "Arquivo .tar"}</small></span>
                <span>{formatDuration(deployment.duration_ms)}</span>
                <span>{formatTime(deployment.created_at)}</span>
              </article>
            ))}
          </div>
        )}
        {result.total > result.per_page && <footer className="pagination"><button className="button secondary" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}>Anterior</button><span>Página {page} de {totalPages}</span><button className="button secondary" disabled={page >= totalPages} onClick={() => setPage((current) => current + 1)}>Próxima</button></footer>}
      </section>
    </>
  );
}
