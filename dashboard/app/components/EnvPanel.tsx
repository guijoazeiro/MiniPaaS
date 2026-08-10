import type { FormEvent } from "react";
import type { EnvKey } from "../types";

type Props = {
  envKeys: EnvKey[];
  envName: string;
  envValue: string;
  saving: boolean;
  deletingKey: string;
  onNameChange: (value: string) => void;
  onValueChange: (value: string) => void;
  onSave: (event: FormEvent<HTMLFormElement>) => void;
  onDelete: (key: string) => void;
};

export function EnvPanel({ envKeys, envName, envValue, saving, deletingKey, onNameChange, onValueChange, onSave, onDelete }: Props) {
  return (
    <section id="environment" className="panel glass env-panel">
      <div className="section-heading"><div><p className="eyebrow">CONFIGURAÇÃO</p><h2>Variáveis de ambiente</h2></div><span className="count">{envKeys.length}</span></div>
      <form className="env-form" onSubmit={onSave}>
        <input value={envName} onChange={(event) => onNameChange(event.target.value)} required placeholder="NOME_DA_VARIÁVEL" />
        <input value={envValue} onChange={(event) => onValueChange(event.target.value)} required placeholder="valor" />
        <button className="button secondary" disabled={saving}>{saving ? "Salvando…" : "Adicionar"}</button>
      </form>
      <div className="env-list">
        {envKeys.length === 0 ? <p className="empty-copy">Nenhuma variável configurada.</p> : envKeys.map((item) => (
          <div className="env-row" key={item.key}>
            <code>{item.key}</code><span>••••••••</span>
            <button className="icon-button" onClick={() => onDelete(item.key)} disabled={Boolean(deletingKey)} aria-label={`Remover ${item.key}`}>{deletingKey === item.key ? "…" : "×"}</button>
          </div>
        ))}
      </div>
    </section>
  );
}
