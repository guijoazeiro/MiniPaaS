import { useCallback, useEffect, useState } from "react";
import { request } from "../lib/api";
import type { App, Deployment, EnvKey } from "../types";

export function useApps(authenticated: boolean, onError: (message: string) => void) {
  const [apps, setApps] = useState<App[]>([]);
  const [selectedName, setSelectedName] = useState("");
  const [selectedApp, setSelectedApp] = useState<App | null>(null);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [envKeys, setEnvKeys] = useState<EnvKey[]>([]);

  const refreshApps = useCallback(async () => {
    const nextApps = await request<App[]>("/apps");
    setApps(nextApps);
    setSelectedName((current) => current || nextApps[0]?.name || "");
  }, []);

  const refreshApp = useCallback(async (name: string) => {
    const encodedName = encodeURIComponent(name);
    const [app, nextDeployments, nextEnv] = await Promise.all([
      request<App>(`/apps/${encodedName}`),
      request<Deployment[]>(`/apps/${encodedName}/deployments`),
      request<EnvKey[]>(`/apps/${encodedName}/env`),
    ]);
    setSelectedApp(app);
    setDeployments(nextDeployments);
    setEnvKeys(nextEnv);
  }, []);

  useEffect(() => {
    if (!authenticated || !selectedName) return;
    const timer = window.setTimeout(() => {
      refreshApp(selectedName)
        .then(() => onError(""))
        .catch((cause: unknown) => {
          onError(cause instanceof Error ? cause.message : "Não foi possível conectar à API.");
        });
    }, 0);
    return () => window.clearTimeout(timer);
  }, [authenticated, onError, refreshApp, selectedName]);

  useEffect(() => {
    if (!authenticated || !selectedName) return;
    const timer = window.setInterval(() => {
      refreshApps()
        .then(() => onError(""))
        .catch((cause: unknown) => {
          onError(cause instanceof Error ? cause.message : "Não foi possível conectar à API.");
        });
      refreshApp(selectedName).catch(() => undefined);
    }, 5000);
    return () => window.clearInterval(timer);
  }, [authenticated, onError, refreshApp, refreshApps, selectedName]);

  const resetApps = useCallback(() => {
    setApps([]);
    setSelectedName("");
    setSelectedApp(null);
    setDeployments([]);
    setEnvKeys([]);
  }, []);

  return {
    apps,
    selectedName,
    selectedApp,
    deployments,
    envKeys,
    setSelectedName,
    refreshApps,
    refreshApp,
    resetApps,
  };
}
