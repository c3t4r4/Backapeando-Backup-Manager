ALTER TABLE admin_users DROP CONSTRAINT IF EXISTS admin_users_cpf_unique;
ALTER TABLE admin_users DROP CONSTRAINT IF EXISTS admin_users_cpf_format;
ALTER TABLE admin_users DROP COLUMN IF EXISTS cpf;
