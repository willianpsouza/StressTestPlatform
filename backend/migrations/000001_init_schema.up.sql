-- Enums
CREATE TYPE user_role AS ENUM ('ROOT', 'USER');
CREATE TYPE user_status AS ENUM ('ACTIVE', 'INACTIVE', 'SUSPENDED');
CREATE TYPE test_status AS ENUM ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED', 'TIMEOUT');
CREATE TYPE schedule_type AS ENUM ('ONCE', 'RECURRING');
CREATE TYPE schedule_status AS ENUM ('ACTIVE', 'PAUSED', 'COMPLETED', 'CANCELLED');

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name VARCHAR(255) NOT NULL,
    role user_role NOT NULL DEFAULT 'USER',
    status user_status NOT NULL DEFAULT 'ACTIVE',
    grafana_user_id INTEGER,
    grafana_username VARCHAR(255),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;

-- Sessions
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    user_agent TEXT,
    ip_address VARCHAR(45),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);

-- Domains
CREATE TABLE domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(user_id, name)
);

CREATE INDEX idx_domains_user_id ON domains(user_id) WHERE deleted_at IS NULL;

-- Tests
CREATE TABLE tests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    script_filename VARCHAR(255) NOT NULL,
    script_path TEXT NOT NULL,
    script_size_bytes BIGINT NOT NULL DEFAULT 0,
    default_vus INTEGER NOT NULL DEFAULT 1,
    default_duration VARCHAR(50) NOT NULL DEFAULT '30s',
    influxdb_bucket VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(domain_id, name)
);

CREATE INDEX idx_tests_domain_id ON tests(domain_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_tests_user_id ON tests(user_id) WHERE deleted_at IS NULL;

-- Test Executions
CREATE TABLE test_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    schedule_id UUID,
    vus INTEGER NOT NULL DEFAULT 1,
    duration VARCHAR(50) NOT NULL DEFAULT '30s',
    status test_status NOT NULL DEFAULT 'PENDING',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    exit_code INTEGER,
    stdout TEXT,
    stderr TEXT,
    metrics_summary JSONB,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_test_executions_test_id ON test_executions(test_id);
CREATE INDEX idx_test_executions_user_id ON test_executions(user_id);
CREATE INDEX idx_test_executions_status ON test_executions(status);

-- Schedules
CREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    schedule_type schedule_type NOT NULL,
    cron_expression VARCHAR(100),
    next_run_at TIMESTAMPTZ,
    vus INTEGER NOT NULL DEFAULT 1,
    duration VARCHAR(50) NOT NULL DEFAULT '30s',
    status schedule_status NOT NULL DEFAULT 'ACTIVE',
    last_run_at TIMESTAMPTZ,
    run_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_schedules_test_id ON schedules(test_id);
CREATE INDEX idx_schedules_status_next_run ON schedules(status, next_run_at) WHERE status = 'ACTIVE';
