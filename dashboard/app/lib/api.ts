const apiUrl = (process.env.NEXT_PUBLIC_MINIPAAS_API_URL || "http://localhost:8080").replace(/\/$/, "");

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = "ApiError";
  }
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${apiUrl}${path}`, { credentials: "include", ...init });
  if (!response.ok) {
    const body = await response.json().catch(() => null);
    throw new ApiError(response.status, body?.error || `A API respondeu ${response.status}.`);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export function websocketUrl(path: string) {
  return `${apiUrl.replace(/^http/, "ws")}${path}`;
}

export function formatTime(value?: string) {
  return value
    ? new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(new Date(value))
    : "—";
}

export function formatDuration(value?: number) {
  if (!value) return "—";
  return value < 1000 ? `${value} ms` : `${(value / 1000).toFixed(1)} s`;
}

const statusLabels: Record<string, string> = {
  running: "Em execução",
  failed: "Falhou",
  idle: "Sem deploy",
  pending: "Na fila",
  building: "Construindo",
  superseded: "Substituído",
  rolled_back: "Revertido",
};

export function stateLabel(status: string) {
  return statusLabels[status] || status;
}
