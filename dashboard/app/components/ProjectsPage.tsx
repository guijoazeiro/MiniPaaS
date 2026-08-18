"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { formatTime, stateLabel, request } from "../lib/api";
import type { CapacitySnapshot, Deployment, DeploymentPage } from "../types";
import { useDashboard } from "./DashboardShell";

export function ProjectsPage() {
  const { apps, openNewProject, refreshApps, setFeedback } = useDashboard();
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [refreshing, setRefreshing] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [capacity, setCapacity] = useState<CapacitySnapshot | null>(null);
  const deploymentRequest = useRef<Promise<void> | null>(null);

  const loadDeployments = useCallback(() => {
    if (deploymentRequest.current) return deploymentRequest.current;

    const promise = request<DeploymentPage>("/deployments?per_page=200")
      .then((page) => {
        setDeployments(page.items);
        setLastUpdated(new Date());
      });
    deploymentRequest.current = promise;
    promise.then(
      () => { if (deploymentRequest.current === promise) deploymentRequest.current = null; },
      () => { if (deploymentRequest.current === promise) deploymentRequest.current = null; },
    );
    return promise;
  }, []);

  const loadCapacity = useCallback(() => request<CapacitySnapshot>("/capacity").then(setCapacity), []);

  useEffect(() => {
    const initialTimer = window.setTimeout(() => {
      void loadDeployments().catch(() => undefined);
      void loadCapacity().catch(() => undefined);
    }, 0);
    const timer = window.setInterval(() => void loadDeployments().catch(() => undefined), 5000);
    const capacityTimer = window.setInterval(() => void loadCapacity().catch(() => undefined), 5000);
    return () => { window.clearTimeout(initialTimer); window.clearInterval(timer); window.clearInterval(capacityTimer); };
  }, [loadCapacity, loadDeployments]);

  const refreshProjects = useCallback(async () => {
    if (refreshing) return;
    setRefreshing(true);
    try {
      await Promise.all([refreshApps(), loadDeployments(), loadCapacity()]);
      setLastUpdated(new Date());
    } catch (cause: unknown) {
      setFeedback(cause instanceof Error ? cause.message : "Não foi possível atualizar os projetos.", "error");
    } finally {
      setRefreshing(false);
    }
  }, [loadCapacity, loadDeployments, refreshApps, refreshing, setFeedback]);

  const latestByApp = useMemo(() => {
    const latest = new Map<string, Deployment>();
    for (const deployment of deployments) {
      if (deployment.app_name && !latest.has(deployment.app_name)) latest.set(deployment.app_name, deployment);
    }
    return latest;
  }, [deployments]);

  const running = apps.filter((app) => app.status === "running").length;
  const attention = apps.filter((app) => app.status === "failed").length;
  const stopped = apps.filter((app) => app.status === "stopped").length;

  return (
    <>
      <header className="page-heading">
        <div><p className="eyebrow">SEUS SERVIÇOS</p><h1>Projetos</h1><p className="page-description">Selecione uma aplicação para acompanhar sua operação e configurar novos releases.</p></div>
        <button className="button primary" onClick={openNewProject}>Criar projeto</button>
      </header>

      <section className="metric-grid" aria-label="Resumo dos projetos">
        <article><span>Total</span><strong>{apps.length}</strong><small>projetos cadastrados</small></article>
        <article><span>Em execução</span><strong>{running}</strong><small>containers ativos</small></article>
        <article><span>Requer atenção</span><strong>{attention}</strong><small>projetos com falha</small></article>
        <article><span>Parados</span><strong>{stopped}</strong><small>execução interrompida</small></article>
      </section>

      {capacity && <section className="panel capacity-summary" aria-label="Capacidade da plataforma">
        <div className="section-heading">
          <div><p className="eyebrow">OPERAÇÃO</p><h2>Capacidade da plataforma</h2><p>Acompanhamento leve da fila e dos limites configurados.</p></div>
          <small className="muted">Atualizado junto com os projetos</small>
        </div>
        <div className="capacity-grid">
          <article><span>Builds ativos</span><strong>{capacity.builds.active}/{capacity.builds.limit}</strong><small>{capacity.builds.queued ? `${capacity.builds.queued} na fila` : "fila vazia"}</small></article>
          <article><span>Aplicações</span><strong>{capacity.apps_total}{capacity.max_apps_per_user ? `/${capacity.max_apps_per_user}` : ""}</strong><small>{capacity.apps_running} em execução</small></article>
          <article><span>Fila de deployments</span><strong>{capacity.builds.queued}</strong><small>{capacity.builds.queued === 1 ? "deployment aguardando" : "deployments aguardando"}</small></article>
        </div>
      </section>}

      <section className="panel project-directory">
        <div className="directory-heading">
          <div><h2>Todos os projetos</h2><p>Visão geral da sua plataforma.</p></div>
          <div className="project-heading-actions">
            <small className="muted" aria-live="polite">
              {refreshing ? "Atualizando…" : lastUpdated ? `Atualizado às ${lastUpdated.toLocaleTimeString("pt-BR")}` : "Aguardando atualização"}
            </small>
            <span className="count">{apps.length}</span>
            <button className="button secondary" type="button" onClick={() => void refreshProjects()} disabled={refreshing} aria-label="Atualizar projetos">
              {refreshing ? "Atualizando…" : "Atualizar"}
            </button>
          </div>
        </div>
        {apps.length === 0 ? (
          <div className="empty-state roomy"><p className="eyebrow">PRIMEIRO PROJETO</p><h2>Sua plataforma está pronta.</h2><p>Crie uma aplicação para configurar a origem e iniciar o primeiro deploy.</p><button className="button primary" onClick={openNewProject}>Criar projeto</button></div>
        ) : (
          <div className="project-table">
            <div className="project-table-head"><span>Projeto</span><span>Estado</span><span>Origem</span><span>Último deployment</span><span>Atualização</span><span /></div>
            {apps.map((app) => {
              const latest = latestByApp.get(app.name);
              const source = latest?.repository ? `${latest.repository}@${latest.branch || "main"}` : latest ? "Upload manual" : "Sem origem";
              return (
                <Link className="project-row" key={app.id} href={`/dashboard/projects/${encodeURIComponent(app.name)}`}>
                  <span className="project-identity"><i className={`status-dot ${app.status}`} /><span><strong>{app.name}</strong><small>{app.public_url || "URL ainda indisponível"}</small></span></span>
                  <span><span className={`status-pill ${app.status}`}><i />{stateLabel(app.status)}</span></span>
                  <span className="truncate-cell">{source}</span>
                  <span>{latest ? stateLabel(latest.status) : "Nenhum"}</span>
                  <span>{formatTime(app.updated_at)}</span>
                  <span className="row-action">Abrir</span>
                </Link>
              );
            })}
          </div>
        )}
      </section>
    </>
  );
}
