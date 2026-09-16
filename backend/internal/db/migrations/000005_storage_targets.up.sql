CREATE TABLE storage_targets (
    id                              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                            text NOT NULL,
    type                            text NOT NULL CHECK (type IN ('azure', 's3', 'filesystem')),
    azure_account_name              text,
    azure_container_name            text,
    azure_sas_token_encrypted       bytea,
    azure_sas_token_expires_at      timestamptz,
    s3_endpoint                     text,
    s3_region                       text,
    s3_bucket                       text,
    s3_access_key_id                text,
    s3_secret_access_key_encrypted  bytea,
    s3_use_path_style               boolean NOT NULL DEFAULT false,
    fs_root_path                    text,
    created_at                      timestamptz NOT NULL DEFAULT now(),
    updated_at                      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT storage_targets_azure_fields CHECK (
        type <> 'azure' OR (azure_account_name IS NOT NULL AND azure_container_name IS NOT NULL AND azure_sas_token_encrypted IS NOT NULL)
    ),
    CONSTRAINT storage_targets_s3_fields CHECK (
        type <> 's3' OR (s3_bucket IS NOT NULL AND s3_access_key_id IS NOT NULL AND s3_secret_access_key_encrypted IS NOT NULL)
    ),
    CONSTRAINT storage_targets_fs_fields CHECK (
        type <> 'filesystem' OR fs_root_path IS NOT NULL
    )
);

-- Preserve azure_targets.id exactly: the AES-256-GCM AAD used to encrypt
-- sas_token_encrypted is `id + ":" + "sas_token"` (see internal/crypto and
-- internal/storage.AzureSASTokenAAD). Changing the id here would make every
-- already-saved SAS token permanently undecryptable.
INSERT INTO storage_targets (id, name, type, azure_account_name, azure_container_name, azure_sas_token_encrypted, azure_sas_token_expires_at, created_at, updated_at)
SELECT id, name, 'azure', account_name, container_name, sas_token_encrypted, sas_token_expires_at, created_at, updated_at
FROM azure_targets;

ALTER TABLE servers ADD COLUMN storage_target_id uuid REFERENCES storage_targets(id);
UPDATE servers SET storage_target_id = azure_target_id WHERE azure_target_id IS NOT NULL;

-- azure_targets and servers.azure_target_id are intentionally NOT dropped
-- here: they remain as an unused safety net until a future, separate
-- migration removes them after a production validation period.
