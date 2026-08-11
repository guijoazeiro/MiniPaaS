# MiniPaaS Dashboard

Painel web para operar a API do MiniPaaS: autenticação, aplicações, deploys, logs em tempo real, rollback e variáveis de ambiente.

## Desenvolvimento local

1. Suba a API do MiniPaaS em `http://localhost:8080`.
2. Copie `.env.example` para `.env.local` caso a API use outro endereço e ajuste `NEXT_PUBLIC_MINIPAAS_API_URL`.
3. Execute `npm install` e `npm run dev`.

O frontend usa a sessão por cookie emitida por `POST /auth/web-login`, sem expor o token ao JavaScript. Configure `DASHBOARD_ORIGIN` na API com a origem do dashboard (por padrão, `http://localhost:3000`). O CLI continua usando `POST /auth/login`.

## Verificação

`npm run build` gera a versão de produção e `npm test` também verifica a renderização inicial.
