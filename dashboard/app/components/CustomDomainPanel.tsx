"use client";

import type { FormEvent } from "react";
import type { CustomDomain } from "../types";

type Props = {
  domains: CustomDomain[];
  hostname: string;
  saving: boolean;
  verifyingID: string;
  deletingID: string;
  onHostnameChange: (value: string) => void;
  onAdd: (event: FormEvent<HTMLFormElement>) => void;
  onVerify: (id: string) => void;
  onDelete: (id: string) => void;
};

const statusLabel: Record<CustomDomain["status"], string> = {
  pending: "Aguardando DNS",
  verified: "Verificado",
  active: "Ativo",
  error: "Erro",
};

export function CustomDomainPanel({ domains, hostname, saving, verifyingID, deletingID, onHostnameChange, onAdd, onVerify, onDelete }: Props) {
  return (
    <section className="panel glass custom-domain-panel">
      <div className="section-heading"><div><p className="eyebrow">REDE</p><h2>Domínios customizados</h2></div><span className="count">{domains.length}</span></div>
      <p className="panel-description">Conecte um domínio próprio. Depois de criar o registro DNS, verifique o domínio para ativar a rota e o HTTPS.</p>
      <form className="custom-domain-form" onSubmit={onAdd}>
        <input value={hostname} onChange={(event) => onHostnameChange(event.target.value)} required placeholder="api.exemplo.com" aria-label="Domínio customizado" />
        <button className="button secondary" disabled={saving}>{saving ? "Adicionando…" : "Adicionar domínio"}</button>
      </form>
      <div className="custom-domain-list">
        {domains.length === 0 ? <p className="empty-copy">Nenhum domínio customizado configurado.</p> : domains.map((item) => (
          <div className="custom-domain-row" key={item.id}>
            <div className="custom-domain-copy"><strong>{item.hostname}</strong>{item.last_error && <small>{item.last_error}</small>}</div>
            <span className={`status-pill compact ${item.status}`}><i />{statusLabel[item.status]}</span>
            {(item.status === "pending" || item.status === "error" || item.status === "verified") && <button type="button" className="text-button" onClick={() => onVerify(item.id)} disabled={Boolean(verifyingID)}>{verifyingID === item.id ? "Verificando…" : "Verificar"}</button>}
            <button type="button" className="text-button danger" onClick={() => onDelete(item.id)} disabled={Boolean(deletingID)}>{deletingID === item.id ? "Removendo…" : "Remover"}</button>
          </div>
        ))}
      </div>
    </section>
  );
}
