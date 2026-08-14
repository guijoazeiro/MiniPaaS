"use client";

import { useState } from "react";

type Props = {
  name: string;
  deleting: boolean;
  onDelete: () => void;
};

export function DeleteAppPanel({ name, deleting, onDelete }: Props) {
  const [confirmation, setConfirmation] = useState("");
  const matches = confirmation.trim() === name;

  return (
    <section className="panel danger-zone">
      <div className="section-heading"><div><p className="eyebrow">ZONA DE PERIGO</p><h2>Excluir aplicação</h2></div></div>
      <p className="panel-description">A aplicação, seus deployments, variáveis, domínios e histórico serão removidos permanentemente. Essa ação não pode ser desfeita.</p>
      <label>Digite <code>{name}</code> para confirmar<input value={confirmation} onChange={(event) => setConfirmation(event.target.value)} placeholder={name} /></label>
      <button className="button danger" disabled={!matches || deleting} onClick={onDelete}>{deleting ? "Excluindo…" : "Excluir aplicação permanentemente"}</button>
    </section>
  );
}
