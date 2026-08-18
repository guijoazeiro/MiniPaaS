# MiniPaaS

[Read this documentation in English](README.md)

Um control plane de deploy self-hosted, inspirado em Render e Railway. O MiniPaaS faz deploy de aplicações baseadas em Dockerfile a partir de um arquivo local ou do GitHub, publica as aplicações atrás do Caddy com HTTPS automático e expõe todo o ciclo de vida por uma CLI e por um dashboard.

Este é um projeto educacional de portfólio com escopo focado em um único host e uma superfície operacional pequena. Ainda assim, ele exercita padrões comuns de produtos reais de infraestrutura: deploys, integração com GitHub, promoção zero-downtime, variáveis de ambiente criptografadas, logs e métricas em tempo real, isolamento entre usuários, API tokens, limites de capacidade e fila de deploys.

## O que este projeto demonstra

- **Arquitetura de control plane:** handlers Gin, serviços de aplicação, tipos de domínio, stores PostgreSQL, migrations versionadas e queries geradas pelo sqlc em camadas separadas.
- **Orquestração de containers:** build de imagens Docker, containers candidatos, verificações de prontidão, políticas de restart, limpeza, rollback e promoção segura de rotas sem Kubernetes.
- **Workflows assíncronos:** deploys em background com cancelamento por contexto, limite de concorrência de builds, fila FIFO, retry e cancelamento.
- **Configuração segura:** valores de ambiente criptografados com AES-256-GCM, credenciais com bcrypt, sessões do dashboard em cookies HTTP-only e automação com tokens mpat_ com escopos, armazenados somente como hashes.
- **Operação em tempo real:** WebSockets para logs e métricas Docker compartilhadas; eventos de build persistidos para manter o histórico de deploys consultável.
- **Isolamento multiusuário:** aplicações, deploys, logs, métricas, instalações GitHub, domínios e eventos de auditoria são vinculados ao usuário proprietário.
- **Integrações de infraestrutura:** a Admin API do Caddy gerencia rotas em runtime, enquanto GitHub Apps e webhooks assinados permitem repositórios privados e auto-deploy.
- **Segurança operacional:** request IDs, auditoria, rate limiting, probes de readiness, health checks, limites de recursos, quotas por usuário, snapshots de capacidade e reconciliação de containers órfãos.

## Stack

| Camada | Escolha | Motivo |
|---|---|---|
| API | Go + Gin | Concorrência adequada a deploys e logs; binário único |
| Banco | PostgreSQL 16 | Simples e robusto, com gen_random_uuid() e BYTEA para segredos criptografados |
| Acesso a dados | sqlc | SQL tipado, sem magia de runtime de ORM |
| Migrations | golang-migrate | Histórico versionado e reversível |
| Runtime | Docker Engine API (Go SDK) | Sem Kubernetes; adequado a um VPS pequeno |
| Proxy reverso | Caddy (Admin API) | Alteração de rotas em runtime e ACME automático |
| Autenticação | JWT (HS256) + bcrypt | Criptografia da biblioteca padrão e sem armazenamento de sessão |
| Segredos | AES-256-GCM | Criptografia autenticada com nonce aleatório por registro |
| Logs | gorilla/websocket + docker/stdcopy | stdout/stderr multiplexados em frames JSON por linha |
| Métricas | Docker stats + gorilla/websocket | Snapshots compartilhados para gráficos de CPU, memória, rede e disco |
| CLI | Go + Cobra | Binário único e simples de distribuir |
| Dashboard | Next.js 16 / Vinext (App Router) | Control plane baseado em rotas usando a mesma API da CLI |

A mesma API HTTP é o contrato do dashboard, da CLI, dos webhooks do GitHub e da automação CI/CD. Assim, autenticação, ownership, validação e comportamento de deploy permanecem consistentes em todas as entradas.

## Mapa de funcionalidades

| Área | Capacidades incluídas |
|---|---|
| Deploys | Upload de tar, repositórios GitHub públicos/privados, validação de Dockerfile, logs de build, retry, cancelamento, rollback e promoção zero-downtime |
| Runtime | Políticas de restart Docker, readiness TCP, subdomínios automáticos, ciclo de vida de rotas Caddy, domínios customizados e HTTPS |
| Observabilidade | Logs stdout/stderr ao vivo, eventos de build persistidos, streams de métricas Docker, métricas de projeto, falhas de health check, request IDs e auditoria |
| Segurança | Login JWT, sessão HTTP-only do dashboard, senhas bcrypt, isolamento por ownership, AES-256-GCM, API tokens com escopos e rate limits |
| Operações | Fila FIFO de builds, quota de aplicações por usuário, limites de CPU/memória/PIDs, endpoint de capacidade e reconciliação no startup |
| Interfaces | CLI Cobra, dashboard para projetos/deploys/logs/métricas/conta, GitHub App e exemplos de CI/CD |

## Início rápido

Pré-requisitos: Go 1.26+, Docker Desktop/Engine, Caddy 2.x e sqlc.

~~~~bash
git clone https://github.com/guijoazeiro/minipaas
cd minipaas
# gere uma chave real: openssl rand -hex 32 e use o resultado em ENCRYPTION_KEY
~~~~

Copie a configuração de exemplo:

~~~~bash
cp .env.example .env
~~~~

Na primeira inicialização, a API cria o usuário administrador a partir de ADMIN_USERNAME e ADMIN_PASSWORD. Consulte .env.example.

## Tutorial completo

O fluxo abaixo vai de uma máquina vazia até minip logs hello -f transmitindo logs. Os comandos usam shell POSIX e funcionam em Linux, macOS e Git Bash no Windows.

### 1. Inicie o PostgreSQL

~~~~bash
docker compose up -d
docker compose ps
~~~~

### 2. Inicie o Caddy

Em outro terminal, mantenha o processo executando:

~~~~bash
caddy run --config Caddyfile
~~~~

O Caddyfile habilita somente a Admin API em localhost:2019. A API adiciona as rotas em runtime e garante que a aplicação HTTP e o servidor srv0 existam.

### 3. Gere o sqlc e execute as migrations

O código tipado fica em api/internal/store/postgres/sqlc/. Regenere-o depois de alterações em api/sql/:

~~~~bash
cd api
sqlc generate
go run ./cmd/migrate up
~~~~

O resultado esperado é migrate ok cmd=up.

### 4. Inicie a API

~~~~bash
go run ./cmd/server
~~~~

Os primeiros logs normalmente incluem:

~~~~text
seeded admin user username=admin
http listening addr=:8080
~~~~

Teste em outro terminal:

~~~~bash
curl localhost:8080/health
# {"status":"ok"}
~~~~

### 5. Inicie o dashboard

~~~~bash
cd dashboard
npm install
npm run dev
~~~~

Abra http://localhost:3000. O dashboard usa um cookie HTTP-only criado por POST /auth/web-login; o token não fica exposto ao JavaScript. Mantenha DASHBOARD_ORIGIN=http://localhost:3000. Se a API estiver em outro endereço, crie dashboard/.env.local:

~~~~env
NEXT_PUBLIC_MINIPAAS_API_URL=http://localhost:8080
~~~~

### 6. Compile a CLI

~~~~bash
cd cli
go build -o minip .
~~~~

### 7. Faça login

~~~~bash
./minip login
~~~~

Quando solicitado, aceite http://localhost:8080 como host e informe ADMIN_USERNAME/ADMIN_PASSWORD. O sucesso aparece como logged in as admin. O token é salvo em %AppData%\minip\config.json no Windows e ~/.config/minip/config.json no Linux/macOS.

### 8. Crie uma aplicação e uma variável

~~~~bash
./minip apps create hello
./minip env set hello GREETING="hello from minipaas"
./minip env list hello
~~~~

env list exibe somente a chave e updated_at. Os valores nunca são retornados pela API. Para conferir o armazenamento:

~~~~bash
docker compose exec postgres psql -U postgres -d minipaas -c "SELECT key, encode(value,'hex') FROM env_vars;"
~~~~

A coluna value sempre contém ciphertext, nunca texto puro.

### 9. Faça deploy da aplicação de exemplo

hello-world/ é uma aplicação Node.js mínima que registra um heartbeat a cada segundo no stdout e um evento a cada três segundos no stderr:

~~~~bash
./minip deploy ../hello-world --app hello --wait
~~~~

O resultado termina com algo como running on host port 57123, a porta do host mapeada para a porta 8080 do container.

### 10. Teste o container

~~~~bash
curl localhost:<porta informada no passo 9>
# hello
~~~~

### 11. Acompanhe os logs

~~~~bash
./minip logs hello
./minip logs hello -f
~~~~

Enquanto -f estiver ativo, faça curl localhost:<porta>/foo em outro terminal. A linha GET /foo aparece ao vivo. Para separar os streams:

~~~~bash
./minip logs hello -f 2>/dev/null  # somente stdout
./minip logs hello -f 1>/dev/null  # somente stderr
~~~~

Se o container falhar, minip logs ainda recupera a saída persistida do último deploy com erro.

### 12. Inspecione a aplicação

~~~~bash
./minip apps info hello
~~~~

O comando mostra status, estado do container, URL https://hello.<BASE_DOMAIN> e deploys recentes.

### 13. Teste os health checks

Para acelerar o teste local, coloque no .env e reinicie a API:

~~~~env
HEALTH_CHECK_INTERVAL=3s
DEPLOY_READY_TIMEOUT=60s
RESTART_POLICY=on-failure
RESTART_MAX_RETRIES=3
~~~~

~~~~bash
container="$(docker ps --filter "name=minipaas-hello" --format "{{.Names}}" | head -n 1)"
docker inspect --format '{{.HostConfig.RestartPolicy.Name}} max={{.HostConfig.RestartPolicy.MaximumRetryCount}}' "$container"
# on-failure max=3
~~~~

Simule falhas e inspecione o resultado:

~~~~bash
for i in 1 2 3 4; do
  docker kill "$container"
  sleep 1
done
./minip apps info hello
docker inspect --format '{{.State.Status}} restartCount={{.RestartCount}}' "$container"
~~~~

O esperado é app e deployment em failed, com container: exited.

### 14. Faça rollback

Altere uma mensagem em hello-world/server.js, faça outro deploy e então execute:

~~~~bash
./minip deploy ../hello-world --app hello --wait
./minip rollback hello
./minip logs hello -f
./minip apps info hello
~~~~

Forma não interativa:

~~~~bash
./minip rollback hello --to <deployment-id>
~~~~

O rollback reutiliza a imagem Docker armazenada. O candidato é validado antes da troca de rota e o container anterior só é parado após a promoção.

### 15. Limpe o ambiente

~~~~bash
./minip apps list
# Ctrl+C no terminal da API e no terminal do Caddy
docker compose down
# docker compose down -v também apaga o volume do banco
~~~~

Para remover uma aplicação e seu histórico:

~~~~bash
./minip apps delete hello --yes
~~~~

## API

Endpoints protegidos exigem Authorization: Bearer <token> ou o cookie HTTP-only do dashboard. Públicos: /health, /ready, /auth/login, /auth/web-login, /auth/register, /auth/logout e o webhook GitHub assinado.

~~~~text
GET    /health                         -> { status: "ok" }
GET    /ready                          -> prontidão do banco, Docker e Caddy
POST   /auth/login                     { username, password } -> { token, expires_at }
POST   /auth/register                  { username, password } -> User
GET    /me                             -> usuário sem hash da senha
PATCH  /me                             { username } -> User
PATCH  /me/password                    { current_password, new_password } -> 204
POST   /me/tokens                      { name, scopes?, expires_at? } -> token uma vez
GET    /me/tokens                      -> metadados dos tokens
DELETE /me/tokens/:id                  -> 204
POST   /apps                            { name } -> App
GET    /apps                            -> []App
GET    /capacity                        -> capacidade e fila
GET    /apps/:name                      -> App e estado do container
DELETE /apps/:name                      -> 204
POST   /apps/:name/stop                 -> 204
GET    /apps/:name/metrics              -> snapshot de métricas
GET    /apps/:name/domains              -> []CustomDomain
POST   /apps/:name/domains              { hostname } -> CustomDomain
POST   /apps/:name/domains/:id/verify   -> CustomDomain
DELETE /apps/:name/domains/:id          -> 204
POST   /apps/:name/deployments          multipart source=<tar> -> 202
POST   /apps/:name/deployments/git      { branch? } -> 202
GET    /apps/:name/deployments          -> []Deployment
GET    /apps/:name/deployments/:id      -> Deployment
GET    /deployments?page=1&per_page=50&app=&status= -> página de deploys
GET    /audit?limit=50&offset=0         -> eventos de auditoria
PUT    /apps/:name/source/git           { repository, branch?, build_context?, dockerfile_path? } -> GitSource
GET    /apps/:name/source/git           -> GitSource
DELETE /apps/:name/source/git           -> 204
GET    /apps/:name/env                  -> chaves e timestamps
PUT    /apps/:name/env/:key             { value } -> 204
DELETE /apps/:name/env/:key             -> 204
POST   /apps/:name/rollback             { deployment_id } -> Deployment
POST   /apps/:name/deployments/:id/retry  -> Deployment
POST   /apps/:name/deployments/:id/cancel -> Deployment
WS     /apps/:name/logs?follow=true&tail=100
WS     /apps/:name/metrics/stream
GET    /apps/:name/deployments/:id/logs?after=0&limit=500
~~~~

O cadastro aceita usernames de 3 a 64 caracteres e senhas com pelo menos 8 caracteres. Aplicações, deploys, logs, variáveis, domínios, métricas e auditoria respeitam o ownership do usuário autenticado.

### API tokens para automação e CI/CD

Tokens pessoais opacos usam o prefixo mpat_ e pelo menos 256 bits de aleatoriedade criptograficamente segura. O MiniPaaS armazena somente o hash SHA-256 e um prefixo curto para identificação. O segredo bruto é exibido uma única vez na criação.

O gerenciamento exige uma sessão JWT/cookie:

~~~~text
POST   /me/tokens       { name, scopes?, expires_at? } -> metadados + segredo uma vez
GET    /me/tokens       -> somente metadados
DELETE /me/tokens/:id   -> revogação imediata
~~~~

Escopos disponíveis:

- read: leitura de projetos, deploys, logs, métricas, capacidade, fontes Git, chaves de ambiente, domínios, auditoria e metadados do GitHub;
- deploy: upload ou disparo de deploys, retry/cancelamento e rollback;
- manage: criação, parada e exclusão de aplicações, fontes Git, variáveis e domínios.

Tokens usam deny-by-default e nunca aumentam as permissões do usuário proprietário. Alterações de perfil, credenciais, tokens e instalações GitHub continuam exigindo sessão.

Para a CLI e CI/CD, defina MINIPAAS_TOKEN. Ele tem prioridade sobre o token salvo pelo minip login e não é persistido automaticamente:

~~~~bash
export MINIPAAS_HOST="http://localhost:8080"
export MINIPAAS_TOKEN="mpat_..."
./minip apps list
./minip deploy --git --app hello --wait
~~~~

~~~~bash
./minip tokens create ci-deploy --scope read --scope deploy --expires-at 2027-01-01T00:00:00Z
./minip tokens list
./minip tokens revoke <token-id> --yes
~~~~

Guarde o segredo em um password manager ou secret store de CI/CD. Nunca o inclua no código, em issues ou nos logs do runner.

Exemplo de GitHub Actions para uma aplicação já conectada ao repositório no MiniPaaS. Cadastre MINIPAAS_HOST e MINIPAAS_TOKEN como secrets do repositório:

~~~~yaml
name: Deploy MiniPaaS
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    env:
      MINIPAAS_HOST: SEU_HOST_MINIPAAS
      MINIPAAS_TOKEN: SEU_TOKEN_EM_SECRET
    steps:
      - name: Disparar deploy
        run: |
          curl --fail-with-body --silent --show-error \
            --request POST \
            --header "Authorization: Bearer $MINIPAAS_TOKEN" \
            --header "Content-Type: application/json" \
            "$MINIPAAS_HOST/apps/hello/deployments/git"
~~~~

## Dashboard

O dashboard oferece rotas dedicadas para:

- **Projetos:** lista aplicações, status, origem, último deploy, quota e capacidade;
- **Deployments:** histórico global com filtros;
- **Logs:** escolha uma aplicação e acompanhe stdout/stderr;
- **Métricas:** gráficos em tempo real de CPU, memória, rede e disco;
- **Conta:** perfil, senha, tokens API e instalações do GitHub App.

A página de projetos faz polling leve, sem reload completo, para manter status e capacidade atualizados. O dashboard usa a mesma API autenticada da CLI.

## CLI

~~~~text
minip login
minip apps create <name>
minip apps list
minip apps info <name>
minip apps stop <name>
minip apps delete <name> --yes
minip deploy <path> --app <name> --wait
minip deploy --git --app <name> --wait
minip logs <name> [--tail 100] [-f]
minip rollback <name> [--to <deployment-id>]
minip tokens create <name> --scope read --scope deploy
minip tokens list
minip tokens revoke <id> --yes
~~~~

O arquivo de configuração fica em %AppData%\minip\config.json no Windows e ~/.config/minip/config.json no Linux/macOS. Ele é criado com permissão restrita. MINIPAAS_TOKEN sempre tem prioridade sem substituir o token salvo.

## Integração com GitHub

### Deploy de repositório público

Configure a origem Git na aplicação com repository, branch, build_context e dockerfile_path. O Dockerfile deve estar dentro do contexto de build; por padrão, é Dockerfile na raiz. O deploy clona o commit informado, valida o contexto e executa o build com limite de tamanho e timeout.

### Repositórios privados com GitHub App

Crie um GitHub App com acesso de leitura a Contents e Metadata, configure a URL pública do App e o webhook com GITHUB_WEBHOOK_SECRET. O arquivo PEM deve ficar fora do repositório e ser apontado por GITHUB_APP_PRIVATE_KEY_PATH. Cada usuário pode instalar o App em sua própria conta ou organização; as instalações ficam vinculadas ao usuário MiniPaaS.

O dashboard mostra as instalações disponíveis e permite trocar a instalação usada pelo deploy. O MiniPaaS usa tokens efêmeros do GitHub App e nunca salva a credencial do repositório.

### Auto-deploy por webhook assinado

Configure o webhook do GitHub para apontar ao endpoint público e selecione o evento Push. O servidor valida a assinatura HMAC-SHA256, encontra a aplicação vinculada ao repositório/branch e dispara um novo deploy. O endpoint possui rate limit próprio.

## Deploys e runtime

### Logs de build persistidos

Eventos de build são persistidos no PostgreSQL e podem ser consultados depois que o WebSocket termina:

~~~~text
GET /apps/:name/deployments/:id/logs?after=0&limit=500
~~~~

O dashboard combina stream ao vivo e histórico persistido, evitando perda de contexto em deploys já finalizados.

### Retry e cancelamento

Deploys Git em estado failed podem ser repetidos pelo dashboard ou pela CLI. Deploys pending ou building podem ser cancelados. O retry cria uma nova tentativa e preserva o histórico original; o cancelamento respeita os contextos de build e remove waiters da fila quando aplicável.

### Zero-downtime

O deploy cria uma imagem e um container candidato, aguarda a prontidão TCP e só então troca a rota do Caddy. O container anterior permanece atendendo enquanto o candidato é validado. Se a validação ou a publicação falhar, a versão ativa continua preservada.

Em rollback, a imagem existente é reutilizada. Metadados de candidato permitem que uma reinicialização remova candidatos órfãos e restaure a última rota confirmada.

### Domínios customizados

Aplicações podem ter hostnames customizados além de <app>.<BASE_DOMAIN>:

~~~~text
GET    /apps/:name/domains
POST   /apps/:name/domains                 { "hostname": "api.example.com" }
POST   /apps/:name/domains/:id/verify
DELETE /apps/:name/domains/:id
~~~~

Crie o registro DNS antes de verificar. O hostname é normalizado com IDNA/punycode. Quando o DNS aponta para o servidor e as portas 80/443 estão acessíveis, o Caddy pode emitir HTTPS automaticamente. Em produção, defina PUBLIC_IP para validação estrita.

Teste local com nip.io:

~~~~bash
nslookup api.127.0.0.1.nip.io
curl -i -H "Host: api.127.0.0.1.nip.io" http://localhost/
~~~~

### Métricas operacionais

GET /apps/:name/metrics retorna CPU, memória, uptime, restarts, resumo de deploys, duração média e falhas recentes de health check. A visão do projeto mostra um snapshot atualizado periodicamente e a página Métricas apresenta gráficos.

### Métricas em tempo real

A página de métricas abre um WebSocket autenticado:

~~~~text
GET /apps/:name/metrics/stream
~~~~

O backend compartilha um stream Docker por container entre visualizadores. Os dados incluem CPU, memória, rede, disco, PIDs, uptime e restart count. O dashboard mantém uma janela móvel de amostras, reconecta automaticamente e usa o snapshot REST como fallback inicial. Nenhuma série histórica é persistida nesta fase.

Para gerar carga temporária em outro terminal:

~~~~bash
docker ps --filter "name=minipaas-nodetest" --format "{{.Names}}"
docker exec <nome-do-container> node -e "const end=Date.now()+30000; while(Date.now()<end){}"
~~~~

### Capacidade, fila de deploys e observabilidade

MAX_APPS_PER_USER limita o número de aplicações de cada usuário; 0 desativa a quota. Ao atingir o limite, a API responde 429.

Os builds usam um scheduler FIFO em memória limitado por MAX_CONCURRENT_BUILDS. Enquanto aguardam uma vaga, os deploys ficam pending e aparecem como **Na fila** no dashboard e na CLI. Cancelar um deploy pendente remove o waiter sem interromper builds ativos.

~~~~text
GET /capacity
~~~~

O endpoint retorna contagem de aplicações do usuário, quota configurada, builds ativos e na fila, limite de concorrência e limites de recursos dos containers. A página de projetos atualiza esses dados sem recarregar a página.

Como o projeto é single-host, a fila é deliberadamente local ao processo. O PostgreSQL continua sendo a fonte de verdade dos registros de deploy. Fila distribuída, retenção longa de métricas, alertas, papéis de equipe e múltiplos hosts ficam fora do escopo final.

## Estrutura do projeto

~~~~text
minipaas/
├── api/
│   ├── cmd/server/                    # entrada HTTP Gin
│   ├── cmd/migrate/                   # executor de migrations
│   ├── internal/config/               # ambiente -> configuração
│   ├── internal/domain/               # tipos e erros de domínio
│   ├── internal/store/postgres/       # stores concretos e sqlc
│   ├── internal/docker/               # wrapper do Docker Engine
│   ├── internal/caddy/                # wrapper da Admin API do Caddy
│   ├── internal/crypto/               # AES-256-GCM
│   ├── internal/health/               # inspeção periódica
│   ├── internal/ws/                   # WebSockets de logs e métricas
│   ├── internal/service/              # autenticação, deploys e integrações
│   └── internal/handler/              # handlers Gin e middleware JWT
├── cli/                               # módulo Go separado, CLI Cobra
├── dashboard/                         # control plane Next.js
├── hello-world/                       # aplicação Node.js de exemplo
├── Caddyfile                          # configuração mínima do Caddy
├── docker-compose.yml                 # PostgreSQL local
└── .env.example
~~~~

## Configuração

Todas as configurações são variáveis de ambiente carregadas pelo .env. O servidor e o migrate procuram .env no diretório atual e um nível acima, então é possível executar a partir da raiz ou de api/.

| Variável | Obrigatória | Padrão | Finalidade |
|---|---|---|---|
| DATABASE_URL | sim | — | DSN do PostgreSQL |
| BASE_DOMAIN | sim | — | Ex.: minipaas.seudominio.com |
| PUBLIC_IP | não | — | IP usado na validação de DNS |
| ENCRYPTION_KEY | sim | — | Chave hexadecimal de 32 bytes |
| JWT_SECRET | sim | — | Segredo de assinatura JWT |
| PORT | não | :8080 | Endereço HTTP |
| DOCKER_HOST | não | autodetectado | Socket/npipe do Docker |
| CADDY_ADMIN_URL | não | http://localhost:2019 | Deve ser loopback |
| TOKEN_TTL | não | 24h | Duração do JWT |
| ADMIN_USERNAME | não | — | Usuário criado no primeiro startup |
| ADMIN_PASSWORD | não | — | Senha criada no primeiro startup |
| IMAGE_RETENTION | não | 5 | Imagens recentes mantidas por aplicação |
| HEALTH_CHECK_INTERVAL | não | 30s | Intervalo da inspeção de containers |
| DEPLOY_READY_TIMEOUT | não | 60s | Timeout de prontidão do candidato |
| MAX_DEPLOY_SIZE_MB | não | 100 | Tamanho máximo do upload |
| MAX_REPOSITORY_SIZE_MB | não | 250 | Tamanho máximo do contexto Git |
| GIT_CLONE_TIMEOUT | não | 10m | Timeout do clone GitHub |
| BUILD_TIMEOUT | não | 15m | Timeout de um build Docker |
| MAX_CONCURRENT_BUILDS | não | 2 | Builds simultâneos; os demais entram na fila |
| MAX_APPS_PER_USER | não | 20 | Quota de aplicações; 0 desativa |
| GITHUB_APP_ID | condicional | — | ID numérico do GitHub App |
| GITHUB_APP_SLUG | condicional | — | Slug do GitHub App |
| GITHUB_APP_PRIVATE_KEY_PATH | condicional | — | Caminho absoluto do PEM fora do repo |
| GITHUB_WEBHOOK_SECRET | não | — | Segredo dos webhooks assinados |
| RESTART_POLICY | não | on-failure | Política de restart Docker |
| RESTART_MAX_RETRIES | não | 3 | Máximo de retries on-failure |
| DASHBOARD_ORIGIN | não | http://localhost:3000 | Origem permitida para o cookie |
| RATE_LIMIT_WINDOW | não | 1m | Janela do rate limiter |
| AUTH_RATE_LIMIT | não | 10 | Limite de login por endereço |
| WEBHOOK_RATE_LIMIT | não | 120 | Limite de webhooks por endereço |
| CONTAINER_MEMORY_LIMIT_MB | não | 0 | Limite opcional de memória |
| CONTAINER_NANO_CPUS | não | 0 | Limite opcional de CPU |
| CONTAINER_PIDS_LIMIT | não | 0 | Limite opcional de processos |
| READINESS_TIMEOUT | não | 3s | Timeout das probes de readiness |
| LOG_LEVEL | não | info | debug, info, warn ou error |

As respostas da API incluem X-Request-ID. O rate limiter é por processo e em memória; em uma instalação com múltiplas instâncias, ele deve ser movido para um store compartilhado.

Limites de memória, NanoCPUs e PIDs são aplicados em deploys e rollbacks. O endpoint autenticado /capacity mostra builds ativos, fila, quantidade de aplicações e limites configurados. No startup, a API reconcilia containers com a label com.minipaas.managed=true e remove somente órfãos; containers externos nunca são tocados.

### Backup e restore do PostgreSQL

Mantenha backups fora do Git. Com o Compose ativo:

~~~~bash
mkdir -p backups
docker compose exec -T postgres pg_dump -U postgres -d minipaas --format=custom > "backups/minipaas-$(date +%Y%m%d-%H%M%S).dump"
~~~~

Para restaurar em um banco descartável:

~~~~bash
docker compose exec -T postgres createdb -U postgres minipaas_restore
docker compose cp backups/minipaas-YYYYMMDD-HHMMSS.dump postgres:/tmp/minipaas.restore.dump
docker compose exec -T postgres pg_restore -U postgres -d minipaas_restore --clean --if-exists /tmp/minipaas.restore.dump
~~~~

Sempre valide migrations e testes de integração depois de um restore. Um backup só é confiável depois que a restauração foi exercitada.

### Trilha de auditoria

Cada requisição HTTP mutável (POST, PUT, PATCH ou DELETE) gera um evento no PostgreSQL após terminar. O registro contém usuário, rota, status, request ID e timestamp. Corpos de requisição nunca são persistidos, portanto senhas, valores de ambiente e fontes não entram na auditoria. Consulte com GET /audit?limit=50&offset=0.

## Desenvolvimento

~~~~bash
cd api
sqlc generate
go run ./cmd/migrate up
go run ./cmd/migrate down 1
go run ./cmd/migrate version
go run ./cmd/migrate force <v>
go build -o bin/server ./cmd/server
go build -o bin/migrate ./cmd/migrate
cd ../cli
go build -o minip .
~~~~

### Testes

Os testes padrão são unitários e não precisam de Docker ou PostgreSQL. O tutorial completo continua sendo o caminho manual de validação com Docker, PostgreSQL e Caddy.

~~~~bash
# API, a partir de api/
go test ./...
go vet ./...

# CLI
cd cli && go test ./...

# Dashboard
cd dashboard
npm test
npm run lint
npm run build

# Integração PostgreSQL, com Compose e migrations aplicados
DATABASE_URL='postgres://postgres:postgres@localhost:5432/minipaas?sslmode=disable' go test -count=1 -tags=integration ./api/internal/store/postgres
~~~~

A cobertura inclui crypto, autenticação, isolamento de ambientes, Caddy, WebSockets de logs e métricas, request IDs, rate limit, readiness, quotas, fila FIFO, reconciliação de containers, health checks, tarball e stores PostgreSQL.

O teste -race precisa de CGO e é executado no Linux pelo CI. O GitHub Actions também executa vet, testes da CLI e do dashboard, migrations e a suíte de integração PostgreSQL.

### CI e releases

O repositório possui dois workflows:

- **CI:** valida a API, executa testes com race no Linux, aplica migrations em PostgreSQL, roda integração, testa a CLI e verifica lint/build do dashboard.
- **Release:** quando uma tag v* é enviada, publica binários da API, migrate e CLI para Linux amd64, macOS amd64/arm64 e Windows amd64.

### Artefatos de release

Uma tag como v0.1.0 inicia o workflow de release e cria uma release do GitHub com os binários multiplataforma.

## Limitações conhecidas

O projeto foi desenhado para aprendizado em um único host. A fila de builds e o rate limiter são locais ao processo, não há retenção histórica de métricas, RBAC de equipes, scheduler multi-host ou provisionamento automático de máquinas. O readiness padrão verifica conectividade TCP; health checks HTTP por caminho são uma extensão futura.

## Licença

MIT
