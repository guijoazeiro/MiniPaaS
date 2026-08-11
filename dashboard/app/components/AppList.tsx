import type { App } from "../types";
import { stateLabel } from "../lib/api";

type Props = {
  apps: App[];
  selectedName: string;
  onSelect: (name: string) => void;
};

export function AppList({ apps, selectedName, onSelect }: Props) {
  return (
    <div className="app-list glass">
      <div className="section-heading"><div><p className="eyebrow">SEUS SERVIÇOS</p><h2>Aplicações</h2></div><span className="count">{apps.length}</span></div>
      <div className="app-items">
        {apps.length === 0 ? <p className="empty-copy">Crie a primeira aplicação para começar.</p> : apps.map((app) => (
          <button key={app.id} className={`app-item ${app.name === selectedName ? "selected" : ""}`} onClick={() => onSelect(app.name)}>
            <span className={`status-dot ${app.status}`} />
            <span className="app-name">{app.name}<small>{app.container_state || stateLabel(app.status)}</small></span>
            <span className="chevron">›</span>
          </button>
        ))}
      </div>
    </div>
  );
}
