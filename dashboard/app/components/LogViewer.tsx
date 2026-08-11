import type { RefObject } from "react";

type Props = {
  logs: string[];
  outputRef: RefObject<HTMLPreElement | null>;
};

export function LogViewer({ logs, outputRef }: Props) {
  return (
    <section className="panel glass logs-panel">
      <div className="section-heading"><div><p className="eyebrow">STREAM AO VIVO</p><h2>Logs</h2></div><span className="live"><i /> AO VIVO</span></div>
      <pre ref={outputRef} className="log-output">{logs.length ? logs.join("\n") : "Aguardando eventos do container…"}</pre>
    </section>
  );
}
