-- Add CPF (Brazilian taxpayer ID) as a mandatory, unique identifier for admin users.
-- Format is validated at the application layer (backend/internal/cpf), including
-- the official check-digit algorithm; the CHECK here is a defense-in-depth
-- guard against anything writing to this column outside the Go application.

ALTER TABLE admin_users ADD COLUMN cpf char(11);

ALTER TABLE admin_users
    ADD CONSTRAINT admin_users_cpf_format CHECK (cpf ~ '^[0-9]{11}$');

ALTER TABLE admin_users
    ADD CONSTRAINT admin_users_cpf_unique UNIQUE (cpf);

ALTER TABLE admin_users ALTER COLUMN cpf SET NOT NULL;
