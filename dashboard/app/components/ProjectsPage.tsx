"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { formatTime, stateLabel, request } from "../lib/api";
import type { Deployment, DeploymentPage } from "../types";
import { useDashboard } from "./DashboardShell";

export function ProjectsPage() {
  const { apps, openNewProject, setFeedback } = useDashboard();
  const [deployments, setDeployments] = useState<Deployment[]>([]);

  const loadDeployments = useCallback(() => {
    return request<DeploymentPage>("/deployments?per_page=200")
      .then((page) => setDeployments(page.items))
      .catch((cause: unknown) => setFeedback(cause instanceof Error ? cause.message : "Não foi possível carregar os deployments.", "error"));
  }, [setFeedback]);

  useEffect(() => {
    const initialTimer = window.setTimeout(() => void loadDeployments(), 0);
    const timer = window.setInterval(() => void loadDeployments(), 5000);
    return () => { window.clearTimeout(initialTimer); window.clearInterval(timer); };
  }, [loadDeployments]);

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

      <section className="panel project-directory">
        <div className="directory-heading"><div><h2>Todos os projetos</h2><p>Visão geral da sua plataforma.</p></div><span className="count">{apps.length}</span></div>
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
