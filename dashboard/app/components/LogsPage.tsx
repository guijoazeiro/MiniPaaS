"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useLogStream } from "../hooks/useLogStream";
import { useDashboard } from "./DashboardShell";
import { LogViewer } from "./LogViewer";

export function LogsPage() {
  const { apps } = useDashboard();
  const router = useRouter();
  const searchParams = useSearchParams();
  const selectedName = searchParams.get("app") || "";
  const { logs, outputRef, following, connection, handleScroll, resumeFollowing, clearLogs } = useLogStream(true, selectedName);

  function selectApp(name: string) {
    router.replace(name ? `/dashboard/logs?app=${encodeURIComponent(name)}` : "/dashboard/logs");
  }

  return (
    <>
      <header className="page-heading logs-page-heading">
        <div><p className="eyebrow">OBSERVABILIDADE</p><h1>Logs</h1><p className="page-description">Acompanhe o stream de uma aplicação sem perder sua posição de leitura.</p></div>
        <label className="project-selector">Projeto<select value={selectedName} onChange={(event) => selectApp(event.target.value)}><option value="">Selecione uma aplicação</option>{apps.map((app) => <option key={app.id} value={app.name}>{app.name}</option>)}</select></label>
      </header>
      <LogViewer logs={logs} outputRef={outputRef} following={following} connection={connection} dedicated onScroll={handleScroll} onResume={resumeFollowing} onClear={clearLogs} />
    </>
  );
}
