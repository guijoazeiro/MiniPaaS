"use client";

import type { MetricsPoint } from "../types";

type MetricKey = keyof Pick<MetricsPoint, "cpu_percent" | "memory_percent" | "memory_usage_bytes" | "network_rx_bytes" | "network_tx_bytes" | "block_read_bytes" | "block_write_bytes">;

type Props = {
  title: string;
  subtitle: string;
  points: MetricsPoint[];
  metric: MetricKey;
  color: string;
  max?: number;
  formatValue: (value: number) => string;
};

function chartPath(points: MetricsPoint[], metric: MetricKey, max: number) {
  if (points.length === 0) return "";
  return points.map((point, index) => {
    const x = points.length === 1 ? 0 : (index / (points.length - 1)) * 100;
    const value = Number(point[metric]) || 0;
    const y = 100 - Math.min(100, Math.max(0, value / max * 100));
    return `${x.toFixed(2)},${y.toFixed(2)}`;
  }).join(" ");
}

export function MetricsChart({ title, subtitle, points, metric, color, max: fixedMax, formatValue }: Props) {
  const latest = points[points.length - 1];
  const latestValue = latest ? Number(latest[metric]) || 0 : 0;
  const highest = points.reduce((value, point) => Math.max(value, Number(point[metric]) || 0), 0);
  const max = fixedMax || Math.max(1, highest * 1.15);
  const path = chartPath(points, metric, max);

  return (
    <section className="metrics-chart panel glass">
      <div className="metrics-chart-heading"><div><p className="eyebrow">{subtitle}</p><h3>{title}</h3></div><strong>{formatValue(latestValue)}</strong></div>
      <div className="metrics-chart-canvas">
        <svg viewBox="0 0 100 100" preserveAspectRatio="none" role="img" aria-label={`${title}: ${formatValue(latestValue)}`}>
          {[20, 40, 60, 80].map((y) => <line key={y} x1="0" x2="100" y1={y} y2={y} className="metrics-chart-grid" />)}
          <line x1="0" x2="100" y1="100" y2="100" className="metrics-chart-axis" />
          {path && <polyline points={path} fill="none" stroke={color} strokeWidth="1.5" vectorEffect="non-scaling-stroke" />}
        </svg>
        {points.length === 0 && <span className="metrics-chart-empty">Aguardando dados do container…</span>}
      </div>
      <div className="metrics-chart-scale"><span>0</span><span>{formatValue(max)}</span></div>
    </section>
  );
}
