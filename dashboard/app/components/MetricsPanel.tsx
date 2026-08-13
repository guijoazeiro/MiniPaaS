"use client";

import { formatDuration, formatTime, stateLabel } from "../lib/api";
import type { AppMetrics } from "../types";

function formatBytes(value: number) {
  if (!value) return "—";
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function formatUptime(seconds: number) {
  if (!seconds) return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

export function MetricsPanel({ metrics }: { metrics: AppMetrics | null }) {
  if (!metrics) return null;
  const runtime = metrics.runtime;
  const deploymentSummary = metrics.deployments;
  const successRate = Math.min(100, Math.max(0, deploymentSummary.success_rate));

  return (
    <section className="panel glass metrics-panel">
      <div className="section-heading metrics-panel-heading">
        <div><p className="eyebrow">TELEMETRIA</p><h2>Métricas operacionais</h2></div>
        <small>Atualizado {formatTime(metrics.collected_at)}</small>
      </div>

      <div className="runtime-metric-grid">
        <article><span>CPU</span><strong>{runtime ? `${runtime.cpu_percent.toFixed(2)}%` : "—"}</strong><small>uso instantâneo</small></article>
        <article><span>Memória</span><strong>{runtime ? `${runtime.memory_percent.toFixed(1)}%` : "—"}</strong><small>{runtime ? `${formatBytes(runtime.memory_usage_bytes)} / ${formatBytes(runtime.memory_limit_bytes)}` : "sem snapshot"}</small></article>
        <article><span>Uptime</span><strong>{formatUptime(runtime?.uptime_seconds || 0)}</strong><small>{runtime?.state ? stateLabel(runtime.state) : "sem container"}</small></article>
        <article><span>Restarts</span><strong>{runtime?.restart_count ?? "—"}</strong><small>desde a inicialização</small></article>
      </div>

      <div className="metrics-detail-grid">
        <div className="metrics-deployment-summary">
          <div className="metrics-subheading"><div><p className="eyebrow">RELEASES</p><h3>Saúde dos deployments</h3></div><strong>{deploymentSummary.success_rate.toFixed(0)}%</strong></div>
          <div className="metrics-progress" aria-label={`Taxa de sucesso de ${deploymentSummary.success_rate.toFixed(0)}%`}><span style={{ width: `${successRate}%` }} /></div>
          <div className="metrics-breakdown"><span>{deploymentSummary.successful} bem-sucedidos</span><span>{deploymentSummary.failed} falhos</span><span>{deploymentSummary.in_progress} em andamento</span></div>
          <small className="metrics-caption">{deploymentSummary.total} deployments · média de {formatDuration(deploymentSummary.average_duration_ms)}</small>
        </div>

        <div className="metrics-health-summary">
          <div className="metrics-subheading"><div><p className="eyebrow">HEALTH CHECK</p><h3>Falhas recentes</h3></div><strong>{metrics.health_check_failures.length}</strong></div>
          {metrics.health_check_failures.length === 0 ? <p className="empty-copy">Nenhuma falha registrada.</p> : <div className="metrics-failure-list">{metrics.health_check_failures.map((failure) => <div key={`${failure.deployment_id}-${failure.created_at}`}><span>{failure.message}</span><small>{formatTime(failure.created_at)}</small></div>)}</div>}
        </div>
      </div>
    </section>
  );
}
