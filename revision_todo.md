# Sugestoes Iniciais (para revisao do Claude)

Estas sugestoes foram apontadas no inicio da sessao, com foco em riscos e melhorias.

1. Ajustar CORS: hoje usa `AllowedOrigins: *` com `AllowCredentials: true`, o que e invalido em browsers e amplia superficie de ataque.
1. Remover `/.env` do controle de versao e manter apenas `/.env.example`.
1. Remover `frontend/node_modules` do repo e garantir `.gitignore` adequado.
1. Aplicar configuracoes de pool do Postgres via `pgxpool.ParseConfig` antes de criar o pool.
1. Ajustar `Scheduler.Stop()` para nao bloquear e lidar com multiplas chamadas com seguranca.
1. Reduzir tempo de lock em `K6Runner.Run()` evitando segurar mutex durante chamada ao DB.
1. Adicionar testes automatizados minimos (auth, scheduler, k6 runner).
