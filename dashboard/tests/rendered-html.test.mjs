import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

async function render() {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}`);
  const { default: worker } = await import(workerUrl.href);
  return worker.fetch(
    new Request("http://localhost/", { headers: { accept: "text/html" } }),
    { ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) } },
    { waitUntil() {}, passThroughOnException() {} },
  );
}

test("server-renders the MiniPaaS control plane", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);
  const html = await response.text();
  assert.match(html, /<title>MiniPaaS .* Control Plane<\/title>/i);
  assert.match(html, /Iniciando o painel/);
  assert.doesNotMatch(html, /codex-preview|react-loading-skeleton|Your site is taking shape/i);
});

test("keeps product behavior and style after the dashboard split", async () => {
  const [page, layout, css, packageJson, landing, login, logStream, deploymentList] = await Promise.all([
    readFile(new URL("../app/page.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/layout.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/globals.css", import.meta.url), "utf8"),
    readFile(new URL("../package.json", import.meta.url), "utf8"),
    readFile(new URL("../app/components/Landing.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/components/LoginScreen.tsx", import.meta.url), "utf8"),
    readFile(new URL("../app/hooks/useLogStream.ts", import.meta.url), "utf8"),
    readFile(new URL("../app/components/DeploymentList.tsx", import.meta.url), "utf8"),
  ]);

  assert.match(page, /\/apps\/.*\/deployments/);
  assert.match(page, /minipaas-theme/);
  assert.match(page, /auth\/web-login/);
  assert.match(page, /auth\/logout/);
  assert.match(logStream, /\/logs\?follow=true/);
  assert.match(landing, /function Landing/);
  assert.match(login, /Mostrar senha/);
  assert.match(deploymentList, /\["superseded", "rolled_back", "stopped"\]/);
  assert.match(page, /\/apps\/.*\/stop/);
  assert.match(logStream, /followingRef/);
  assert.match(logStream, /scrollHeight - output\.scrollTop - output\.clientHeight/);
  assert.match(layout, /MiniPaaS .* Control Plane/);
  assert.match(layout, /og\.png/);
  assert.match(css, /backdrop-filter/);
  assert.doesNotMatch(css, /gradient/i);
  assert.doesNotMatch(packageJson, /react-loading-skeleton|drizzle/i);
});
