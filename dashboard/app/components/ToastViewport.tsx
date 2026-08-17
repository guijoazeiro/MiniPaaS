"use client";

import { useEffect } from "react";

export type ToastKind = "success" | "error";

export type ToastNotice = {
  id: number;
  message: string;
  kind: ToastKind;
};

type ToastItemProps = {
  toast: ToastNotice;
  onDismiss: (id: number) => void;
};

function ToastItem({ toast, onDismiss }: ToastItemProps) {
  useEffect(() => {
    if (toast.kind !== "success") return undefined;

    const timer = window.setTimeout(() => onDismiss(toast.id), 5000);
    return () => window.clearTimeout(timer);
  }, [onDismiss, toast.id, toast.kind]);

  return (
    <div className={`toast ${toast.kind}`} role={toast.kind === "error" ? "alert" : "status"}>
      <span>{toast.message}</span>
      <button type="button" className="toast-dismiss" onClick={() => onDismiss(toast.id)} aria-label="Fechar notificação">×</button>
    </div>
  );
}

type ToastViewportProps = {
  toasts: ToastNotice[];
  onDismiss: (id: number) => void;
};

export function ToastViewport({ toasts, onDismiss }: ToastViewportProps) {
  if (toasts.length === 0) return null;

  return (
    <div className="toast-viewport" role="region" aria-label="Notificações">
      {toasts.map((toast) => <ToastItem key={toast.id} toast={toast} onDismiss={onDismiss} />)}
    </div>
  );
}
