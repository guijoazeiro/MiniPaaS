import type { Theme } from "../types";

type Props = {
  theme: Theme;
  onThemeChange: () => void;
  onAccess: () => void;
  apiIssue: string;
};

export function Landing({ theme, onThemeChange, onAccess, apiIssue }: Props) {
  return (
    <main className="landing-shell" data-theme={theme}>
      <header className="landing-nav">
        <div className="brand"><span className="brand-mark">M</span><span>MiniPaaS</span></div>
        <div className="landing-nav-actions">
          <button className="theme-button" onClick={onThemeChange}>{theme === "dark" ? "☼ Claro" : "◐ Escuro"}</button>
          <button className="button secondary" onClick={onAccess}>Acessar painel</button>
        </div>
      </header>

      <section className="landing-hero">
        <div className="hero-copy">
          <p className="eyebrow">SELF-HOSTED DEPLOYMENT PLATFORM</p>
          <h1>Deploy.<br />Observe.<br /><span>Controle.</span></h1>
          <p className="hero-description">Uma plataforma direta para colocar aplicações no ar, acompanhar cada release e manter a operação sob controle.</p>
          {apiIssue && <p className="feedback error api-issue">API indisponível: {apiIssue}</p>}
          <div className="hero-actions">
            <button className="button primary hero-cta" onClick={onAccess}>Entrar no painel <span>→</span></button>
            <a href="#capabilities" className="text-link">Conhecer recursos <span>↓</span></a>
          </div>
          <div className="hero-grid" aria-hidden="true"><i /><i /><i /><i /><i /><i /><i /><i /><i /></div>
        </div>

        <div className="hero-art" aria-label="Prévia do painel MiniPaaS">
          <div className="art-rail"><b>▦</b><span>◇</span><span>◫</span><span>⌁</span><span>⚙</span></div>
          <div className="art-content">
            <div className="art-title"><div><p>APPLICATIONS</p><strong>Seu ambiente</strong></div><button>＋ Nova aplicação</button></div>
            <div className="art-metrics"><div><small>Total</small><strong>12</strong></div><div><small>Running</small><strong>11 <em>● Healthy</em></strong></div><div><small>Stopped</small><strong>1</strong></div></div>
            <div className="art-table">
              <div className="art-head"><span>Application</span><span>Status</span><span>Updated</span></div>
              {["web-api", "worker", "frontend", "cron"].map((name, index) => (
                <div className="art-row" key={name}>
                  <span><i className={index === 3 ? "idle" : ""} />{name}<small>{name}</small></span>
                  <span className={index === 3 ? "stopped" : "healthy"}>● {index === 3 ? "Stopped" : "Running"}</span>
                  <span>{index === 0 ? "2m" : index === 1 ? "5m" : index === 2 ? "12m" : "1h"}</span>
                </div>
              ))}
            </div>
            <div className="art-footer"><span>Ver todas as aplicações →</span><small>System Status <b>● Healthy</b></small></div>
          </div>
        </div>
      </section>

      <section id="capabilities" className="capabilities">
        <article><span>01</span><h2>Deploys sem atrito.</h2><p>Envie seu código e acompanhe a construção em um único lugar.</p></article>
        <article><span>02</span><h2>Observabilidade real.</h2><p>Logs em tempo real e histórico de releases para decisões rápidas.</p></article>
        <article><span>03</span><h2>Controle quando importa.</h2><p>Rollback e variáveis de ambiente sem sair do fluxo.</p></article>
      </section>
    </main>
  );
}
