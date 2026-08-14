"use client";

import { useState, type FormEvent } from "react";
import type { Theme } from "../types";

type Props = {
  theme: Theme;
  onThemeChange: () => void;
  onBack: () => void;
  onLogin: () => void;
  onRegister: (event: FormEvent<HTMLFormElement>) => void;
  error: string;
  apiIssue: string;
  busy: boolean;
};

export function RegisterScreen({ theme, onThemeChange, onBack, onLogin, onRegister, error, apiIssue, busy }: Props) {
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmation, setShowConfirmation] = useState(false);

  return (
    <main className="login-shell" data-theme={theme}>
      <section className="login-card glass">
        <button className="back-button" onClick={onBack}>← Voltar</button>
        <div className="brand-mark">M</div>
        <p className="eyebrow">MINIPAAS / NOVA CONTA</p>
        <h1>Comece a operar.</h1>
        <p className="muted">Crie uma conta para organizar suas aplicações, deployments e métricas.</p>
        {apiIssue && <p className="feedback error api-issue">API indisponível: {apiIssue}</p>}
        <form onSubmit={onRegister} className="form-stack">
          <label>Usuário<input name="username" autoComplete="username" required minLength={3} maxLength={64} placeholder="guilherme" /></label>
          <label>
            Senha
            <span className="password-field">
              <input name="password" type={showPassword ? "text" : "password"} autoComplete="new-password" required minLength={8} placeholder="••••••••" />
              <button type="button" onClick={() => setShowPassword((visible) => !visible)} aria-label={showPassword ? "Ocultar senha" : "Mostrar senha"}>{showPassword ? "Ocultar" : "Mostrar"}</button>
            </span>
          </label>
          <label>
            Confirmar senha
            <span className="password-field">
              <input name="password_confirmation" type={showConfirmation ? "text" : "password"} autoComplete="new-password" required minLength={8} placeholder="••••••••" />
              <button type="button" onClick={() => setShowConfirmation((visible) => !visible)} aria-label={showConfirmation ? "Ocultar confirmação" : "Mostrar confirmação"}>{showConfirmation ? "Ocultar" : "Mostrar"}</button>
            </span>
          </label>
          {error && <p className="feedback error">{error}</p>}
          <button className="button primary" disabled={busy}>{busy ? "Criando…" : "Criar conta"}</button>
        </form>
        <p className="auth-switch">Já tem uma conta? <button type="button" onClick={onLogin}>Entrar</button></p>
      </section>
      <button className="theme-button login-theme" onClick={onThemeChange} aria-label="Alternar tema">{theme === "dark" ? "☼ Claro" : "◐ Escuro"}</button>
    </main>
  );
}
