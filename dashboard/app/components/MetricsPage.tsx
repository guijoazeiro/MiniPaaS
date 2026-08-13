"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useMetricsStream } from "../hooks/useMetricsStream";
import { formatTime, stateLabel } from "../lib/api";
import type { App } from "../types";
import { MetricsChart } from "./MetricsChart";
import { useDashboard } from "./DashboardShell";

function formatBytes(value: number) {
  if (!value) return "0 B";
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

function connectionLabel(connection: string) {
  if (connection === "connected") return "AO VIVO";
  if (connection === "retrying") return "RECONECTANDO";
  if (connection === "connecting") return "CONECTANDO";
  return "AGUARDANDO";
}

export function MetricsPage() {
  const { apps } = useDashboard();
  const router = useRouter();
  const searchParams = useSearchParams();
  const selectedName = searchParams.get("app") || "";
  const { current, points, connection } = useMetricsStream(selectedName);
  const runtime = current;

  function selectApp(name: string) {
    router.replace(name ? `/dashboard/metrics?app=${encodeURIComponent(name)}` : "/dashboard/metrics");
  }

  return (
    <>
      <header className="page-heading metrics-page-heading">
        <div><p className="eyebrow">OBSERVABILIDADE</p><h1>Métricas</h1><p className="page-description">Acompanhe o comportamento do container em tempo real, com gráficos inspirados no Docker Desktop.</p></div>
        <div className="metrics-page-controls"><label className="project-selector">Projeto<select value={selectedName} onChange={(event) => selectApp(event.target.value)}><option value="">Selecione uma aplicação</option>{apps.map((app: App) => <option key={app.id} value={app.name}>{app.name}</option>)}</select></label>{selectedName && <span className={`live metrics-live ${connection}`}><i />{connectionLabel(connection)}</span>}</div>
      </header>

      {!selectedName ? <div className="panel metrics-empty"><p className="eyebrow">PRIMEIRO PASSO</p><h2>Selecione uma aplicação.</h2><p>Escolha um projeto para abrir o stream de métricas e acompanhar CPU, memória, rede e disco.</p></div> : !runtime ? <div className="panel metrics-empty"><p className="eyebrow">AGUARDANDO CONTAINER</p><h2>Métricas indisponíveis.</h2><p>Faça um deploy ou inicie a aplicação para começar a receber dados.</p></div> : (
        <>
          <div className="metrics-live-summary panel glass"><div><span>Estado</span><strong className={runtime.state}>{stateLabel(runtime.state)}</strong></div><div><span>Atualizado</span><strong>{formatTime(runtime.ts)}</strong></div><div><span>Processos</span><strong>{runtime.pids || "—"}</strong></div><div><span>Container</span><strong>{runtime.container_id ? runtime.container_id.slice(0, 12) : "—"}</strong></div><Link className="text-button" href={`/dashboard/projects/${encodeURIComponent(selectedName)}`}>Abrir projeto</Link></div>
          <section className="metrics-live-cards" aria-label="Resumo em tempo real">
            <article className="panel glass"><span>CPU</span><strong>{runtime.cpu_percent.toFixed(2)}%</strong><small>uso instantâneo</small></article>
            <article className="panel glass"><span>Memória</span><strong>{runtime.memory_percent.toFixed(2)}%</strong><small>{formatBytes(runtime.memory_usage_bytes)} / {formatBytes(runtime.memory_limit_bytes)}</small></article>
            <article className="panel glass"><span>Uptime</span><strong>{formatUptime(runtime.uptime_seconds)}</strong><small>{runtime.restart_count} restarts</small></article>
            <article className="panel glass"><span>Rede</span><strong>{formatBytes(runtime.network_rx_bytes)}</strong><small>recebidos · {formatBytes(runtime.network_tx_bytes)} enviados</small></article>
          </section>
          <div className="metrics-chart-grid">
            <MetricsChart title="CPU usage" subtitle="PROCESSADOR" points={points} metric="cpu_percent" color="var(--violet)" max={100} formatValue={(value) => `${value.toFixed(2)}%`} />
            <MetricsChart title="Memory usage" subtitle="MEMÓRIA" points={points} metric="memory_percent" color="var(--good)" max={100} formatValue={(value) => `${value.toFixed(2)}%`} />
            <MetricsChart title="Network received" subtitle="NETWORK I/O" points={points} metric="network_rx_bytes" color="var(--violet-ink)" formatValue={formatBytes} />
            <MetricsChart title="Network sent" subtitle="NETWORK I/O" points={points} metric="network_tx_bytes" color="var(--warning)" formatValue={formatBytes} />
            <MetricsChart title="Disk read" subtitle="DISK I/O" points={points} metric="block_read_bytes" color="var(--violet-ink)" formatValue={formatBytes} />
            <MetricsChart title="Disk write" subtitle="DISK I/O" points={points} metric="block_write_bytes" color="var(--warning)" formatValue={formatBytes} />
          </div>
        </>
      )}
    </>
  );
}
