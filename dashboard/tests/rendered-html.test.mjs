import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

async function render(path = "/") {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}-${path}`);
  const { default: worker } = await import(workerUrl.href);
  return worker.fetch(
    new Request(`http://localhost${path}`, { headers: { accept: "text/html" } }),
    { ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) } },
    { waitUntil() {}, passThroughOnException() {} },
  );
}

test("server-renders the public and authenticated entry points", async () => {
  const [landingResponse, loginResponse, dashboardResponse] = await Promise.all([
    render("/"),
    render("/login"),
    render("/dashboard/projects"),
  ]);
  assert.equal(landingResponse.status, 200);
  assert.equal(loginResponse.status, 200);
  assert.equal(dashboardResponse.status, 200);
  const landing = await landingResponse.text();
  const login = await loginResponse.text();
  const dashboard = await dashboardResponse.text();
  assert.match(landing, /Deploy\./);
  assert.match(login, /Deploy sem ruído/);
  assert.match(dashboard, /Iniciando o painel/);
  assert.match(landing, /<title>MiniPaaS .* Control Plane<\/title>/i);
  assert.doesNotMatch(`${landing}${login}${dashboard}`, /codex-preview|react-loading-skeleton|Your site is taking shape/i);
});

test("keeps operations while splitting the dashboard into real routes", async () => {
  const [rootPage, layout, css, packageJson, shell, newAppModal, projectDetails, projectsPage, deploymentsPage, logsPage, login, logStream, deploymentList] = await Promise.all([
    readFile(new URL("../app/page.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/layout.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/globals.css", import.meta.url), "utf8"),
    readFile(new URL("../package.json", import.meta.url), "utf8"),
    readFile(new URL("../app/components/DashboardShell.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/components/NewAppModal.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/components/ProjectDetails.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/components/ProjectsPage.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/components/GlobalDeploymentsPage.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/components/LogsPage.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/components/LoginScreen.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/hooks/useLogStream.ts", import.meta.url), "utf8"),
    readFile(new URL("../app/components/DeploymentList.tsx", import.meta.url), "utf8"),
  ]);

  assert.match(rootPage, /dashboard\/projects/);
  assert.match(shell, /dashboard\/projects/);
  assert.match(shell, /dashboard\/deployments/);
  assert.match(shell, /dashboard\/logs/);
  assert.match(shell, /technical-backdrop/);
  assert.match(shell, /control-grid/);
  assert.match(newAppModal, /role="dialog"/);
  assert.match(newAppModal, /aria-modal="true"/);
  assert.match(newAppModal, /event\.key === "Escape"/);
  assert.match(newAppModal, /querySelectorAll<HTMLElement>/);
  assert.match(newAppModal, /returnFocusRef/);
  assert.match(shell, /auth\/logout/);
  assert.match(projectDetails, /\/apps\/.*\/deployments/);
  assert.match(projectDetails, /\/apps\/.*\/stop/);
  assert.match(projectDetails, /source\/git/);
  assert.match(projectsPage, /Todos os projetos/);
  assert.match(deploymentsPage, /\/deployments\?/);
  assert.match(logsPage, /Selecione uma aplicação/);
  assert.match(login, /Mostrar senha/);
  assert.match(logStream, /\/logs\?follow=true/);
  assert.match(logStream, /followingRef/);
  assert.match(logStream, /scrollHeight - output\.scrollTop - output\.clientHeight/);
  assert.match(deploymentList, /\["superseded", "rolled_back", "stopped"\]/);
  assert.match(layout, /MiniPaaS .* Control Plane/);
  assert.match(layout, /og\.png/);
  assert.match(css, /backdrop-filter/);
  assert.match(css, /body::before/);
  assert.match(css, /filter:\s*blur\(1[4-5]0px\)/);
  assert.match(css, /:root\[data-theme="dark"\] body::before\s*\{[^}]*background:\s*#25262d[^}]*opacity:\s*\.045[^}]*filter:\s*blur\(120px\)/);
  assert.match(css, /backdrop-filter:\s*blur\((?:32|34|36)px\) saturate\(1\.25\)/);
  assert.match(css, /:root\[data-theme="dark"\] \.metric-grid article[\s\S]*?background:\s*rgba\(24, 25, 34, \.32\)/);
  assert.match(css, /technical-backdrop/);
  assert.match(css, /technical-fragments[^}]*filter:\s*blur\(22px\)/);
  assert.match(css, /modal-backdrop[^}]*blur\(12px\) saturate\(1\.05\)/s);
  assert.match(css, /:root\[data-theme="dark"\] \.modal-backdrop[^}]*blur\(14px\) saturate\(1\.15\)/s);
  assert.match(css, /background:\s*rgba\(255, 255, 255, \.68\)/);
  assert.match(css, /:root\[data-theme="dark"\] \.modal[^}]*background:\s*rgba\(18, 19, 27, \.72\)/s);
  assert.doesNotMatch(css, /(?:linear|radial|conic)-gradient/);
  assert.doesNotMatch(css, /\.button[^}]*background:\s*(?:linear|radial)-gradient/is);
  assert.doesNotMatch(packageJson, /react-loading-skeleton|drizzle/i);
});
