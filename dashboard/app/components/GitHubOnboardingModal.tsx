"use client";

import { useEffect, useState } from "react";
import { request } from "../lib/api";
import type { GitHubInstallation, User } from "../types";
import { GitHubIcon } from "./GitHubIcon";

const dismissedKey = (userID: string) => `minipaas.github-onboarding.dismissed.${userID}`;

export function GitHubOnboardingModal() {
  const [userID, setUserID] = useState("");
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let active = true;
    async function checkConnection() {
      try {
        const user = await request<User>("/me");
        if (window.localStorage.getItem(dismissedKey(user.id))) return;
        const status = await request<{ enabled: boolean }>("/integrations/github/status");
        if (!status.enabled) return;
        const installations = await request<GitHubInstallation[]>("/integrations/github/installations");
        if (active && installations.length === 0) {
          setUserID(user.id);
          setOpen(true);
        }
      } catch {
        // The dashboard shell already reports API/authentication failures.
      }
    }
    void checkConnection();
    return () => { active = false; };
  }, []);

  function dismiss() {
    if (userID) window.localStorage.setItem(dismissedKey(userID), "1");
    setOpen(false);
  }

  async function connect() {
    setLoading(true);
    try {
      const response = await request<{ url: string }>("/integrations/github/account-install-url");
      if (userID) window.localStorage.setItem(dismissedKey(userID), "1");
      window.location.assign(response.url);
    } catch {
      setLoading(false);
    }
  }

  if (!open) return null;
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal glass" role="dialog" aria-modal="true" aria-labelledby="github-onboarding-title">
        <button className="modal-close" type="button" onClick={dismiss} aria-label="Fechar">×</button>
        <p className="eyebrow">DEPLOYS MAIS SIMPLES</p>
        <h2 id="github-onboarding-title">Conecte seu GitHub</h2>
        <p className="panel-description">Autorize sua conta ou organização do GitHub e faça deploy de repositórios privados sem copiar tokens ou configurar credenciais manualmente.</p>
        <div className="hero-actions">
          <button className="button primary" type="button" onClick={connect} disabled={loading}><GitHubIcon size={16} /> {loading ? "Abrindo GitHub…" : "Conectar GitHub"}</button>
          <button className="button secondary" type="button" onClick={dismiss}>Agora não</button>
        </div>
      </section>
    </div>
  );
}
