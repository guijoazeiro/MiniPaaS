import type { RefObject, UIEventHandler } from "react";

type Props = {
  logs: string[];
  outputRef: RefObject<HTMLPreElement | null>;
  following: boolean;
  connection?: "idle" | "connecting" | "connected" | "retrying";
  dedicated?: boolean;
  onScroll: UIEventHandler<HTMLPreElement>;
  onResume: () => void;
  onClear?: () => void;
};

export function LogViewer({ logs, outputRef, following, connection = "connected", dedicated = false, onScroll, onResume, onClear }: Props) {
  const connectionLabel = connection === "connected" ? "CONECTADO" : connection === "retrying" ? "RECONECTANDO" : connection === "connecting" ? "CONECTANDO" : "AGUARDANDO";
  return (
    <section className={`panel logs-panel ${dedicated ? "dedicated" : ""}`}>
      <div className="section-heading">
        <div><p className="eyebrow">STREAM AO VIVO</p><h2>Logs</h2></div>
        <div className="log-heading-actions">
          {onClear && <button className="text-button" onClick={onClear} disabled={logs.length === 0}>Limpar visualização</button>}
          <span className={`live ${!following ? "paused" : connection}`}><i /> {!following ? "PAUSADO" : connectionLabel}</span>
        </div>
      </div>
      <div className="log-frame">
        <pre ref={outputRef} className="log-output" onScroll={onScroll}>{logs.length ? logs.join("\n") : connection === "idle" ? "Selecione um projeto para visualizar os logs." : "Aguardando eventos do container…"}</pre>
        {!following && <button className="log-resume" onClick={onResume}>Ir para o final</button>}
      </div>
    </section>
  );
}
