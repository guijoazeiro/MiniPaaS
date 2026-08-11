import type { FormEvent } from "react";

type Props = {
  name: string;
  creating: boolean;
  onNameChange: (name: string) => void;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

export function NewAppModal({ name, creating, onNameChange, onClose, onSubmit }: Props) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal glass" role="dialog" aria-modal="true" aria-labelledby="create-app-title">
        <button className="modal-close" onClick={onClose} aria-label="Fechar">×</button>
        <p className="eyebrow">NOVA APLICAÇÃO</p>
        <h2 id="create-app-title">Dê um nome ao serviço.</h2>
        <p className="muted">Use letras minúsculas, números e hífens.</p>
        <form className="form-stack" onSubmit={onSubmit}>
          <label>Nome da aplicação<input autoFocus value={name} onChange={(event) => onNameChange(event.target.value)} required placeholder="minha-api" pattern="[a-z0-9-]+" /></label>
          <button className="button primary" disabled={creating}>{creating ? "Criando…" : "Criar aplicação"}</button>
        </form>
      </section>
    </div>
  );
}
