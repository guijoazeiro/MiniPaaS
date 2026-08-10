"use client";

import { useState, type FormEvent } from "react";
import type { Theme } from "../types";

type Props = {
  theme: Theme;
  onThemeChange: () => void;
  onBack: () => void;
  onLogin: (event: FormEvent<HTMLFormElement>) => void;
  error: string;
  apiIssue: string;
  busy: boolean;
};

export function LoginScreen({ theme, onThemeChange, onBack, onLogin, error, apiIssue, busy }: Props) {
  const [showPassword, setShowPassword] = useState(false);

  return (
    <main className="login-shell" data-theme={theme}>
      <section className="login-card glass">
        <button className="back-button" onClick={onBack}>← Voltar</button>
        <div className="brand-mark">M</div>
        <p className="eyebrow">MINIPAAS / CONTROL PLANE</p>
        <h1>Deploy sem ruído.</h1>
        <p className="muted">Entre para acompanhar as aplicações, os builds e a operação da sua plataforma.</p>
        {apiIssue && <p className="feedback error api-issue">API indisponível: {apiIssue}</p>}
        <form onSubmit={onLogin} className="form-stack">
          <label>Usuário<input name="username" autoComplete="username" required placeholder="admin" /></label>
          <label>
            Senha
            <span className="password-field">
              <input name="password" type={showPassword ? "text" : "password"} autoComplete="current-password" required placeholder="••••••••" />
              <button type="button" onClick={() => setShowPassword((visible) => !visible)} aria-label={showPassword ? "Ocultar senha" : "Mostrar senha"}>{showPassword ? "Ocultar" : "Mostrar"}</button>
            </span>
          </label>
          {error && <p className="feedback error">{error}</p>}
          <button className="button primary" disabled={busy}>{busy ? "Entrando…" : "Entrar no painel"}</button>
        </form>
      </section>
      <button className="theme-button login-theme" onClick={onThemeChange} aria-label="Alternar tema">{theme === "dark" ? "☼ Claro" : "◐ Escuro"}</button>
    </main>
  );
}
