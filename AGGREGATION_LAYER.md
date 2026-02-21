# Camada de Agregacao k6_metrics_aggregated

## Problema

A tabela `k6_metrics` armazena cada data point cru do CSV do K6. Em 24h com 5 testes
recorrentes, acumulou 4GB+ de dados. Todos os dashboards (Grafana, Report, Analytics,
Dashboard Overview) faziam queries pesadas (`PERCENTILE_CONT`, `SUM`, `AVG` com `GROUP BY`)
sobre essa tabela gigante, causando lentidao severa.

## Solucao

Tabela `k6_metrics_aggregated` com dados pre-agregados. No final de cada teste, a stored
procedure `sp_aggregate_execution_metrics` agrega os dados raw, insere na tabela agregada,
e deleta os dados raw via `sp_cleanup_raw_metrics`.

### Tipos de rows na tabela agregada

| is_summary | url      | Descricao                                    | Uso                        |
|------------|----------|----------------------------------------------|----------------------------|
| FALSE      | any      | Buckets de 1 segundo com percentis           | Timeseries (<=12h range)   |
| TRUE       | NULL     | Resumo global por metric_name por execucao   | Stat cards, timeseries >12h|
| TRUE       | NOT NULL | Resumo por endpoint (url/method/status)      | Tabelas HTTP, erros        |

### Auto-routing por time range (metrics-api)

O metrics-api decide automaticamente qual fonte de dados usar:

- **Time range <= 12h**: usa bucket rows (is_summary=FALSE), re-agrega em intervalos maiores
- **Time range > 12h**: usa summary rows por execucao, cada execucao vira um ponto no grafico

Threshold definido pela constante `longRangeThreshold` em `metrics-api/main.go`.

## Arquivos modificados

### Migrations
- `backend/migrations/000005_k6_metrics_aggregated.up.sql` — Tabela, indexes, SPs
- `backend/migrations/000005_k6_metrics_aggregated.down.sql` — Rollback
- `backend/migrations/000006_backfill_aggregated_metrics.up.sql` — Backfill dados existentes
- `backend/migrations/000006_backfill_aggregated_metrics.down.sql` — No-op

### Backend (Go)
- `backend/internal/domain/metric.go` — Adicionado `AggregateAndCleanup` ao interface
- `backend/internal/adapters/postgres/metric.go` — Implementacao do metodo + `DeleteByExecution` atualizado
- `backend/internal/app/k6runner.go` — Chamada pos-teste (apos ComputeExecutionSummary)

### Metrics-API (Go)
- `metrics-api/main.go` — Todas as 7 queries reescritas:
  - `handleGrafanaStats` — CTE com summaries + buckets
  - `tsHandler` — Aceita dual queries (bucket + summary), auto-routing
  - 9 handlers de timeseries — Cada um com bucket query + summary query
  - `handleTableHTTPRequests` — Endpoint summary rows
  - `handleTableErrors` — Endpoint summary rows com filtro de status
  - `handleDashboardOverview` — Global summary rows
  - `handleDashboardDomain` — Global summary rows com filtro de dominio
  - `handleExecutionStats` — Per-execution summaries + buckets para peak RPS

## Stored Procedures

### sp_aggregate_execution_metrics(p_execution_id UUID)
1. Busca test_id dos dados raw
2. DELETE existing aggregated (idempotencia)
3. INSERT per-second bucket rows (GROUP BY second + metric + endpoint)
4. INSERT global summary rows (GROUP BY metric_name only)
5. INSERT per-endpoint summary rows (GROUP BY metric + url + method + status)
6. Chama sp_cleanup_raw_metrics

### sp_cleanup_raw_metrics(p_execution_id UUID)
- DELETE FROM k6_metrics WHERE execution_id = p_execution_id

## Resultado

- Reducao de ~300-600x em rows consultadas
- Dashboard Grafana: de varios segundos para <1 segundo
- Zero impacto em dados futuros: pipeline automatico no k6runner
- Dados raw eliminados apos agregacao (economia de storage)

## Deploy em instancias existentes

Ver secao de deploy abaixo ou executar:
```bash
docker compose build backend metrics-api
docker compose run --rm migrate
docker compose up -d backend metrics-api
```

A migration 000006 faz o backfill automaticamente. Pode levar alguns minutos dependendo
do volume de dados raw existentes.
