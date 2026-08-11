import type { RefObject, UIEventHandler } from "react";

type Props = {
  logs: string[];
  outputRef: RefObject<HTMLPreElement | null>;
  following: boolean;
  onScroll: UIEventHandler<HTMLPreElement>;
  onResume: () => void;
};

export function LogViewer({ logs, outputRef, following, onScroll, onResume }: Props) {
  return (
    <section className="panel glass logs-panel">
      <div className="section-heading">
        <div><p className="eyebrow">STREAM AO VIVO</p><h2>Logs</h2></div>
        <span className={`live ${following ? "" : "paused"}`}><i /> {following ? "ACOMPANHANDO" : "PAUSADO"}</span>
      </div>
      <div className="log-frame">
        <pre ref={outputRef} className="log-output" onScroll={onScroll}>{logs.length ? logs.join("\n") : "Aguardando eventos do container…"}</pre>
        {!following && <button className="log-resume" onClick={onResume}>Ir para o final ↓</button>}
      </div>
    </section>
  );
}
