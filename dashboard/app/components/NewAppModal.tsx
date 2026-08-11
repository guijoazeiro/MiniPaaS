import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";

type Props = {
  name: string;
  creating: boolean;
  onNameChange: (name: string) => void;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

export function NewAppModal({ name, creating, onNameChange, onClose, onSubmit }: Props) {
  const dialogRef = useRef<HTMLElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const returnFocusRef = useRef<HTMLElement | null>(null);
  const closeTimerRef = useRef<number | null>(null);
  const closingRef = useRef(false);
  const [closing, setClosing] = useState(false);

  const requestClose = useCallback(() => {
    if (closingRef.current) return;
    closingRef.current = true;
    setClosing(true);
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (reducedMotion) {
      onClose();
      return;
    }
    closeTimerRef.current = window.setTimeout(onClose, 180);
  }, [onClose]);

  useEffect(() => {
    returnFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    inputRef.current?.focus();

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        requestClose();
        return;
      }
      if (event.key !== "Tab") return;

      const focusable = dialogRef.current?.querySelectorAll<HTMLElement>("button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href]");
      if (!focusable?.length) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      if (closeTimerRef.current) window.clearTimeout(closeTimerRef.current);
      const returnFocus = returnFocusRef.current;
      if (returnFocus?.isConnected) window.requestAnimationFrame(() => returnFocus.focus());
    };
  }, [requestClose]);

  return (
    <div className={`modal-backdrop ${closing ? "is-closing" : ""}`} role="presentation">
      <section ref={dialogRef} className="modal glass" role="dialog" aria-modal="true" aria-labelledby="create-app-title">
        <button className="modal-close" onClick={requestClose} aria-label="Fechar">×</button>
        <p className="eyebrow">NOVA APLICAÇÃO</p>
        <h2 id="create-app-title">Dê um nome ao serviço.</h2>
        <p className="muted">Use letras minúsculas, números e hífens.</p>
        <form className="form-stack" onSubmit={onSubmit}>
          <label htmlFor="new-app-name">Nome da aplicação<input ref={inputRef} id="new-app-name" autoFocus value={name} onChange={(event) => onNameChange(event.target.value)} required placeholder="minha-api" pattern="[a-z0-9-]+" /></label>
          <button className="button primary" disabled={creating}>{creating ? "Criando…" : "Criar aplicação"}</button>
        </form>
      </section>
    </div>
  );
}
