-- Migration: 000058_open_api
-- External Open API integration tables: per-partner API keys, external
-- user/session mappings for federated identity without WeKnora login.
DO $$ BEGIN RAISE NOTICE '[Migration 000058] Creating Open API tables'; END $$;

CREATE TABLE IF NOT EXISTS open_api_clients (
    id              VARCHAR(36)  PRIMARY KEY,
    tenant_id       INTEGER      NOT NULL,
    name            VARCHAR(128) NOT NULL,
    api_key_hash    VARCHAR(64)  NOT NULL,
    allowed_kb_ids  JSONB        NOT NULL DEFAULT '[]'::jsonb,
    status          VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_open_api_clients_api_key_hash
    ON open_api_clients(api_key_hash)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_open_api_clients_tenant
    ON open_api_clients(tenant_id)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS open_api_user_mappings (
    id               BIGSERIAL    PRIMARY KEY,
    tenant_id        INTEGER      NOT NULL,
    client_id        VARCHAR(36)  NOT NULL,
    external_user_id VARCHAR(255) NOT NULL,
    internal_user_id VARCHAR(320) NOT NULL,
    display_name     VARCHAR(255),
    first_seen_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata         JSONB,
    deleted_at       TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_open_api_user_mappings_unique
    ON open_api_user_mappings(tenant_id, client_id, external_user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_open_api_user_mappings_internal
    ON open_api_user_mappings(internal_user_id)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS open_api_session_mappings (
    id                  BIGSERIAL    PRIMARY KEY,
    tenant_id           INTEGER      NOT NULL,
    client_id           VARCHAR(36)  NOT NULL,
    external_user_id    VARCHAR(255) NOT NULL,
    external_session_id VARCHAR(255) NOT NULL,
    internal_session_id VARCHAR(36)  NOT NULL,
    knowledge_base_id   VARCHAR(36),
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_active_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at          TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_open_api_session_mappings_unique
    ON open_api_session_mappings(tenant_id, client_id, external_user_id, external_session_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_open_api_session_mappings_internal
    ON open_api_session_mappings(internal_session_id)
    WHERE deleted_at IS NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000058] Open API tables ready'; END $$;
