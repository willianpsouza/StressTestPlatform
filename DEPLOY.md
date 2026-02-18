# StressTestPlatform - Deploy from Scratch

Guia para deploy em uma VM Ubuntu com Docker e Docker Compose instalados.

## Pre-requisitos

- Ubuntu 20.04+ com acesso SSH
- Docker Engine 24+
- Docker Compose v2+
- Git

```bash
# Verificar
docker --version
docker compose version
git --version
```

## 1. Clonar o repositorio

```bash
cd /opt
git clone https://github.com/willianpsouza/StressTestPlatform.git
cd StressTestPlatform
```

## 2. Gerar o arquivo .env

O script abaixo gera senhas aleatorias automaticamente:

```bash
# Gerar senhas
DB_PASS=$(openssl rand -base64 24 | tr -dc 'a-zA-Z0-9' | head -c 32)
JWT_SECRET=$(openssl rand -base64 48 | tr -dc 'a-zA-Z0-9' | head -c 64)
GRAFANA_PASS=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)
SEED_PASS=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)

cat > .env << EOF
# Application
APP_ENV=production
APP_NAME=StressTestPlatform
APP_DEBUG=false
PROJECT_NAME=StressTestPlatform

# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Database
POSTGRES_USER=stresstest
POSTGRES_PASSWORD=${DB_PASS}
POSTGRES_DB=stresstest
DATABASE_URL=postgres://stresstest:${DB_PASS}@postgres:5432/stresstest?sslmode=disable

# Redis
REDIS_URL=redis://redis:6379/0

# JWT
JWT_SECRET=${JWT_SECRET}

# Grafana
GRAFANA_URL=http://grafana:3000
GRAFANA_PUBLIC_URL=/grafana
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=${GRAFANA_PASS}
GRAFANA_ADMIN_TOKEN=

# Frontend
NEXT_PUBLIC_API_URL=/api/v1
NEXT_PUBLIC_APP_NAME=StressTestPlatform
NEXT_PUBLIC_PROJECT_NAME=StressTestPlatform
INTERNAL_API_URL=http://backend:8080

# Seed (usuario ROOT inicial)
SEED_ROOT_EMAIL=admin@stresstest.local
SEED_ROOT_PASSWORD=${SEED_PASS}
SEED_ROOT_NAME=Admin
EOF

echo ""
echo "========================================="
echo "  Credenciais geradas (ANOTE-AS!)"
echo "========================================="
echo "PostgreSQL:  stresstest / ${DB_PASS}"
echo "Grafana:     admin / ${GRAFANA_PASS}"
echo "App ROOT:    admin@stresstest.local / ${SEED_PASS}"
echo "========================================="
```

## 3. Subir a infraestrutura (Postgres + Redis)

```bash
docker compose up -d postgres redis
```

Aguardar os healthchecks ficarem healthy:

```bash
docker compose ps
# Espere ate STATUS mostrar (healthy) para postgres e redis
```

## 4. Build de todos os servicos

```bash
docker compose build
```

## 5. Aplicar migrations

```bash
docker compose run --rm migrate
```

Saida esperada:
```
migrate: 000001_init_schema... ok
migrate: 000002_system_settings... ok
migrate: 000003_k6_metrics_remove_influxdb... ok
migrate: 000004_k6_metrics_indexes... ok
```

## 6. Seed do usuario ROOT

```bash
docker compose run --rm seed
```

Saida esperada:
```
Created ROOT user: Admin (admin@stresstest.local)
```

## 7. Subir todos os servicos

```bash
docker compose up -d
```

## 8. Verificar

```bash
# Status de todos os containers
docker compose ps

# Logs do backend (deve mostrar "Server listening on 0.0.0.0:8080")
docker compose logs backend --tail 10

# Logs do frontend (deve mostrar "Ready in Xms")
docker compose logs frontend --tail 5

# Testar health check
curl -s http://localhost/health | python3 -m json.tool
```

Todos os 8 servicos devem estar `Up`:

| Servico | Container | Porta |
|---------|-----------|-------|
| PostgreSQL | stresstest-postgres | 5432 |
| Redis | stresstest-redis | 6379 |
| Backend (Go) | stresstest-backend | - |
| Frontend (Next.js) | stresstest-frontend | - |
| Metrics API (Go) | stresstest-metrics-api | - |
| Grafana | stresstest-grafana | 3001 |
| Dummy API | stresstest-dummy | - |
| Nginx | stresstest-nginx | **80** |

## 9. Acessar

- **Aplicacao:** `http://<IP_DA_VM>`
- **Grafana:** `http://<IP_DA_VM>/grafana/`
- **Login:** usar as credenciais do usuario ROOT exibidas no passo 2

## Comandos uteis

```bash
# Ver logs em tempo real
docker compose logs -f

# Reiniciar um servico especifico
docker compose restart backend

# Rebuild e redeploy de um servico
docker compose up -d --build backend

# Parar tudo
docker compose down

# Parar e remover volumes (APAGA DADOS!)
docker compose down -v

# Reaplicar migrations apos atualizar o codigo
git pull
docker compose build
docker compose run --rm migrate
docker compose up -d
```

## Atualizando para nova versao

```bash
cd /opt/StressTestPlatform
git pull
docker compose up -d --build
# Se houver novas migrations:
docker compose run --rm migrate
```
