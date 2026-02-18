# StressTestPlatform

## Visão Geral
Plataforma para criar, executar e acompanhar testes de carga com K6, com API em Go, frontend em Next.js, métricas persistidas no PostgreSQL, integração com Grafana e um microserviço de métricas para dashboards.

## Componentes e Serviços
- `backend`: API principal (Go) com autenticação, CRUD de domínios/testes/agendamentos, execução de testes K6 e persistência de métricas.
- `metrics-api`: microserviço Go para consultas agregadas e séries temporais usadas no Grafana e no frontend.
- `frontend`: painel web (Next.js) para operação e visualização.
- `grafana`: provisionamento de datasources e dashboard.
- `nginx`: proxy reverso de `/api/v1`, `/grafana` e `/metrics-api`.
- `testapi`: API dummy para servir como alvo de testes K6.
- `postgres`: armazenamento principal.
- `redis`: cache usado pelo `metrics-api`.

## Funcionalidades Disponíveis
### Autenticação e Usuários
- Registro e login com JWT e refresh token.
- Perfil do usuário (nome) e alteração de senha.
- Controle de acesso por roles `ROOT` e `USER`.
- Administração de usuários (somente `ROOT`): listar, editar e remover.

### Domínios
- CRUD de domínios com nome e descrição.
- Listagem com paginação e filtro por busca.
- Vinculação de testes a domínios.

### Testes K6
- CRUD de testes com upload de script `.js`.
- Edição de metadados (nome, descrição, VUs/duração padrão).
- Edição do conteúdo do script via editor no frontend.
- Execução manual com VUs e duração configuráveis.
- Histórico de execuções por teste.

### Execuções
- Criação de execuções por teste.
- Cancelamento de execuções em `PENDING` ou `RUNNING`.
- Consulta de logs (`stdout`/`stderr`).
- Recalcular métricas de uma execução finalizada.
- Remoção de execuções finalizadas e métricas associadas.

### Agendamentos
- Agendamento único (`ONCE`) por data/hora.
- Agendamento recorrente (`RECURRING`) por expressão cron.
- Pausar e retomar agendamentos.
- Execução automática via scheduler.

### Dashboard e Analytics
- Dashboard geral com status de serviços e métricas agregadas.
- Lista global de execuções (todos os usuários).
- Analytics com comparação de duas execuções (K6 stats).

### Grafana
- Provisionamento de datasources (PostgreSQL e Metrics API).
- Dashboard de métricas K6 acessível em `/grafana`.

### Test API (Dummy)
- Endpoints para gerar tráfego e dados de teste.

## API Backend (Base `/api/v1`)
### Formato de Resposta
Todas as respostas seguem o envelope:
```json
{
  "success": true,
  "data": {},
  "error": { "code": "", "message": "", "details": {} },
  "meta": { "total": 0, "page": 1, "page_size": 20, "total_pages": 1 }
}
```

### Endpoints
| Método | Rota | Autenticação | Descrição |
| --- | --- | --- | --- |
| POST | `/auth/register` | Público | Cria usuário e retorna tokens. |
| POST | `/auth/login` | Público | Login e retorno de tokens. |
| POST | `/auth/refresh` | Público | Renova tokens via refresh token. |
| POST | `/auth/logout` | Bearer | Revoga refresh token. |
| GET | `/auth/me` | Bearer | Retorna usuário atual. |
| PUT | `/auth/me` | Bearer | Atualiza perfil (nome). |
| POST | `/auth/change-password` | Bearer | Altera senha do usuário atual. |
| GET | `/domains` | Bearer | Lista domínios (paginação e busca). |
| POST | `/domains` | Bearer | Cria domínio. |
| GET | `/domains/{id}` | Bearer | Detalhe de domínio. |
| PUT | `/domains/{id}` | Bearer | Atualiza domínio. |
| DELETE | `/domains/{id}` | Bearer | Remove domínio. |
| GET | `/tests` | Bearer | Lista testes (paginação, busca, `domain_id`). |
| POST | `/tests` | Bearer | Cria teste (multipart com script). |
| GET | `/tests/{id}` | Bearer | Detalhe de teste. |
| PUT | `/tests/{id}` | Bearer | Atualiza teste (metadados). |
| PUT | `/tests/{id}/script` | Bearer | Substitui script (multipart). |
| GET | `/tests/{id}/script/content` | Bearer | Lê conteúdo do script. |
| PUT | `/tests/{id}/script/content` | Bearer | Salva conteúdo do script. |
| DELETE | `/tests/{id}` | Bearer | Remove teste. |
| GET | `/executions` | Bearer | Lista execuções (paginação, `test_id`, `status`). |
| POST | `/executions` | Bearer | Cria execução para um teste. |
| GET | `/executions/{id}` | Bearer | Detalhe de execução. |
| POST | `/executions/{id}/cancel` | Bearer | Cancela execução `PENDING/RUNNING`. |
| GET | `/executions/{id}/logs` | Bearer | Retorna `stdout`/`stderr`. |
| POST | `/executions/{id}/recalculate-metrics` | Bearer | Recalcula métricas de execução finalizada. |
| DELETE | `/executions/{id}` | Bearer | Remove execução finalizada. |
| DELETE | `/tests/{id}/executions` | Bearer | Remove execuções finalizadas de um teste. |
| GET | `/schedules` | Bearer | Lista agendamentos (paginação, `test_id`, `status`). |
| POST | `/schedules` | Bearer | Cria agendamento. |
| GET | `/schedules/{id}` | Bearer | Detalhe de agendamento. |
| PUT | `/schedules/{id}` | Bearer | Atualiza agendamento. |
| DELETE | `/schedules/{id}` | Bearer | Remove agendamento. |
| POST | `/schedules/{id}/pause` | Bearer | Pausa agendamento. |
| POST | `/schedules/{id}/resume` | Bearer | Retoma agendamento. |
| GET | `/dashboard/executions` | Bearer | Lista global de execuções (todos os usuários). |
| GET | `/dashboard/stats` | Bearer | Estatísticas globais. |
| GET | `/services/status` | Bearer | Status de Postgres, Redis, Grafana, Metrics API e K6. |
| GET | `/users` | Bearer (ROOT) | Lista usuários. |
| GET | `/users/{id}` | Bearer (ROOT) | Detalhe de usuário. |
| PUT | `/users/{id}` | Bearer (ROOT) | Atualiza usuário. |
| DELETE | `/users/{id}` | Bearer (ROOT) | Remove usuário. |
| GET | `/settings` | Bearer (ROOT) | Lê configurações do sistema. |
| PUT | `/settings` | Bearer (ROOT) | Atualiza configurações (ex.: `grafana_token`). |

### Health
- `GET /health`: status e metadata da aplicação.
- `GET /ready`: readiness com checks de Postgres e Redis.

## Metrics API (Base `/metrics-api`)
| Método | Rota | Descrição |
| --- | --- | --- |
| GET | `/health` | Health check simples. |
| GET | `/grafana/variables/domains` | Lista domínios com métricas. |
| GET | `/grafana/variables/tests?domain=` | Lista testes por domínio. |
| GET | `/grafana/stats?domain=&test=&from=&to=&interval=` | Métricas agregadas para Grafana. |
| GET | `/grafana/ts/all` | Série temporal agregada (requests, rps, iterations, response_time, failures). |
| GET | `/grafana/ts/errors` | Série de erros HTTP. |
| GET | `/grafana/ts/response-histogram` | Série de tempo médio de resposta. |
| GET | `/grafana/ts/requests` | Série de requests. |
| GET | `/grafana/ts/vus` | Série de VUs. |
| GET | `/grafana/ts/percentiles` | Série de median/p90/p95. |
| GET | `/grafana/ts/rps` | Série de RPS. |
| GET | `/grafana/ts/iterations` | Série de iterações. |
| GET | `/grafana/ts/req-per-vu` | Série de requests por VU. |
| GET | `/grafana/tables/http-requests` | Tabela HTTP por URL/método/status. |
| GET | `/grafana/tables/errors` | Tabela de erros HTTP. |
| GET | `/dashboard/overview` | Resumo agregado para o dashboard do frontend. |
| GET | `/dashboard/domain?name=` | Resumo agregado por domínio. |
| GET | `/executions/list` | Lista das últimas execuções finalizadas. |
| GET | `/executions/{id}/stats` | Stats agregados de uma execução. |

Parâmetros comuns de tempo aceitam RFC3339, `YYYY-MM-DD` e epoch em ms. O `interval` é em segundos.

## Frontend (Rotas)
- `/login`: login.
- `/register`: registro.
- `/dashboard`: dashboard geral, serviços e execuções recentes.
- `/domains`: lista, criação e edição de domínios.
- `/domains/{id}`: detalhe do domínio e métricas K6 do domínio.
- `/tests`: lista de testes.
- `/tests/new`: criação de teste com upload de script.
- `/tests/{id}`: detalhe do teste, execução manual, editor de script e histórico.
- `/tests/{id}/edit`: edição de metadados do teste.
- `/schedules`: lista de agendamentos.
- `/schedules/new`: criação de agendamento (único ou recorrente).
- `/schedules/{id}`: detalhe do agendamento com pausar/retomar.
- `/executions`: lista de execuções com filtro por status.
- `/executions/{id}`: detalhe da execução, logs e métricas.
- `/analytics`: comparação de duas execuções (dados do metrics-api).
- `/settings`: perfil, alteração de senha e token do Grafana (ROOT).
- `/users`: administração de usuários (ROOT).
- `/grafana`: acesso ao Grafana via proxy.

## Regras e Limites Aplicados
- Senha mínima: 8 caracteres.
- Script K6 deve ser `.js` e ter até 1 MB.
- VUs e duração padrão configuráveis por teste; valores inválidos são ajustados para padrões.
- Limites de execução via env: `K6_MAX_VUS`, `K6_MAX_DURATION`, `K6_MAX_CONCURRENT` (por usuário).
- Agendamento `RECURRING` exige `cron_expression`.
- Agendamento `ONCE` exige `next_run_at`.
- Scheduler executa checks de agendamentos a cada 10s.

## Status e Tipos (Enums)
- `UserRole`: `ROOT`, `USER`.
- `UserStatus`: `ACTIVE`, `INACTIVE`, `SUSPENDED`.
- `TestStatus`: `PENDING`, `RUNNING`, `COMPLETED`, `FAILED`, `CANCELLED`, `TIMEOUT`.
- `ScheduleType`: `ONCE`, `RECURRING`.
- `ScheduleStatus`: `ACTIVE`, `PAUSED`, `COMPLETED`, `CANCELLED`.

## Variáveis de Ambiente (principais)
Definidas em `.env.example`:
- `APP_ENV`, `APP_NAME`, `APP_DEBUG`, `PROJECT_NAME`.
- `SERVER_HOST`, `SERVER_PORT`.
- `DATABASE_URL`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`.
- `REDIS_URL`.
- `JWT_SECRET`.
- `GRAFANA_URL`, `GRAFANA_PUBLIC_URL`, `GRAFANA_ADMIN_USER`, `GRAFANA_ADMIN_PASSWORD`, `GRAFANA_ADMIN_TOKEN`.
- `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_APP_NAME`, `NEXT_PUBLIC_PROJECT_NAME`, `INTERNAL_API_URL`.
- `K6_MAX_DURATION`, `K6_MAX_VUS`, `K6_MAX_CONCURRENT`, `K6_SCRIPTS_PATH` (usados pelo backend).

## Test API (Dummy)
Base `http://dummy:8089`:
- `GET /` health.
- `GET /api/users` dados aleatórios de usuários.
- `GET /api/products` dados aleatórios de produtos.
- `GET /api/orders` dados aleatórios de pedidos.
- `GET /api/slow` resposta com atraso aleatório.
- `POST /api/echo` eco do JSON enviado.

## Proxy (Nginx)
- `/api/v1/` → backend.
- `/metrics-api/` → metrics-api.
- `/grafana/` → Grafana.
- `/health` → backend.
- `/` → frontend.
