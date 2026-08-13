import { useEffect, useState } from "react";
import { request, websocketUrl } from "../lib/api";
import type { AppMetrics, LiveMetricsSnapshot, MetricsPoint } from "../types";

export type MetricsConnection = "idle" | "connecting" | "connected" | "retrying";

function toSnapshot(ts: string, runtime: AppMetrics["runtime"]): LiveMetricsSnapshot | null {
  if (!runtime) return null;
  return {
    ts,
    container_id: runtime.container_id,
    state: runtime.state,
    restart_count: runtime.restart_count,
    uptime_seconds: runtime.uptime_seconds,
    started_at: runtime.started_at,
    cpu_percent: runtime.cpu_percent,
    memory_percent: runtime.memory_percent,
    memory_usage_bytes: runtime.memory_usage_bytes,
    memory_limit_bytes: runtime.memory_limit_bytes,
    network_rx_bytes: runtime.network_rx_bytes || 0,
    network_tx_bytes: runtime.network_tx_bytes || 0,
    block_read_bytes: runtime.block_read_bytes || 0,
    block_write_bytes: runtime.block_write_bytes || 0,
    pids: runtime.pids || 0,
  };
}

function pointFromSnapshot(snapshot: LiveMetricsSnapshot): MetricsPoint {
  return {
    ts: snapshot.ts,
    cpu_percent: snapshot.cpu_percent,
    memory_percent: snapshot.memory_percent,
    memory_usage_bytes: snapshot.memory_usage_bytes,
    network_rx_bytes: snapshot.network_rx_bytes,
    network_tx_bytes: snapshot.network_tx_bytes,
    block_read_bytes: snapshot.block_read_bytes,
    block_write_bytes: snapshot.block_write_bytes,
  };
}

export function useMetricsStream(selectedName: string) {
  const [current, setCurrent] = useState<LiveMetricsSnapshot | null>(null);
  const [points, setPoints] = useState<MetricsPoint[]>([]);
  const [connection, setConnection] = useState<MetricsConnection>("idle");

  useEffect(() => {
    if (!selectedName) {
      const timer = window.setTimeout(() => {
        setCurrent(null);
        setPoints([]);
        setConnection("idle");
      }, 0);
      return () => window.clearTimeout(timer);
    }

    let disposed = false;
    let socket: WebSocket | null = null;
    let retryTimer: number | undefined;

    const addSnapshot = (snapshot: LiveMetricsSnapshot) => {
      if (disposed) return;
      setCurrent(snapshot);
      setPoints((items) => [...items, pointFromSnapshot(snapshot)].slice(-120));
    };

    request<AppMetrics>(`/apps/${encodeURIComponent(selectedName)}/metrics`)
      .then((snapshot) => {
        const runtime = toSnapshot(snapshot.collected_at, snapshot.runtime);
        if (runtime) addSnapshot(runtime);
      })
      .catch(() => undefined);

    const connect = () => {
      if (disposed) return;
      setConnection((value) => value === "connected" ? "retrying" : "connecting");
      socket = new WebSocket(websocketUrl(`/apps/${encodeURIComponent(selectedName)}/metrics/stream`));
      socket.onopen = () => setConnection("connected");
      socket.onmessage = (event) => {
        try {
          const frame = JSON.parse(String(event.data)) as { type?: string; ts?: string; runtime?: AppMetrics["runtime"] };
          if (frame.type !== "metrics" || !frame.ts) return;
          const runtime = toSnapshot(frame.ts, frame.runtime);
          if (runtime) addSnapshot(runtime);
        } catch {
          // Ignore malformed frames and keep the stream alive.
        }
      };
      socket.onclose = () => {
        if (disposed) return;
        setConnection("retrying");
        retryTimer = window.setTimeout(connect, 2000);
      };
      socket.onerror = () => socket?.close();
    };

    connect();
    return () => {
      disposed = true;
      if (retryTimer) window.clearTimeout(retryTimer);
      socket?.close();
    };
  }, [selectedName]);

  return { current, points, connection };
}
