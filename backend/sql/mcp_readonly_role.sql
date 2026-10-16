-- Rol de base de datos de SOLO LECTURA para el servidor MCP (asistentes de IA).
--
-- Se corre UNA VEZ, a mano, como superusuario, en cada base (producción,
-- staging). Es idempotente: se puede volver a correr sin romper nada, por
-- ejemplo después de añadir un tool que lee una tabla nueva.
--
--   psql "$DATABASE_URL_SUPERUSER" -v mcp_password='LA_CONTRASEÑA' -f backend/sql/mcp_readonly_role.sql
--
-- Después, en el backend: MCP_DB_USER=obertrack_mcp_ro y MCP_DB_PASSWORD=<la
-- misma contraseña>, y reiniciar. En el log debe salir
-- "[MCP] Consultas del MCP con el rol de solo lectura obertrack_mcp_ro".
--
-- La contraseña NO va en este archivo: se pasa con -v mcp_password=...
--
-- Qué puede hacer el rol: SELECT en las tablas que usan los tools y, en
-- users, solo en las columnas no sensibles. Nunca: password,
-- obervoice_password, remember_token, reset_token, reset_token_expiry,
-- token_version, identity_document, address, birth_date, emergency_phones,
-- obervoice_qr. Además, toda sesión del rol arranca en solo lectura y con
-- un statement_timeout de 15 s; el backend, por su parte, abre cada consulta
-- con BEGIN READ ONLY.

\set ON_ERROR_STOP on

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'obertrack_mcp_ro') THEN
    CREATE ROLE obertrack_mcp_ro LOGIN;
  END IF;
END
$$;

ALTER ROLE obertrack_mcp_ro WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;
ALTER ROLE obertrack_mcp_ro WITH PASSWORD :'mcp_password';
ALTER ROLE obertrack_mcp_ro SET default_transaction_read_only = on;
ALTER ROLE obertrack_mcp_ro SET statement_timeout = '15s';

DO $$
BEGIN
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO obertrack_mcp_ro', current_database());
END
$$;

GRANT USAGE ON SCHEMA public TO obertrack_mcp_ro;
REVOKE CREATE ON SCHEMA public FROM obertrack_mcp_ro;

-- Parte de una base limpia: quita cualquier permiso previo sobre tablas,
-- para que lo único vigente sea lo que sigue.
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM obertrack_mcp_ro;

-- users: solo columnas no sensibles (las empresas también son filas de users).
GRANT SELECT (
  id, name, email, user_type, is_active, is_manager, deleted_at,
  company_name, industry, client_since, assigned_cs_id,
  job_title, phone_number, country, state
) ON users TO obertrack_mcp_ro;

-- Tablas completas que leen los tools.
GRANT SELECT ON
  employments,
  employment_managers,
  work_hours,
  boards,
  board_members,
  tasks,
  task_users,
  comments,
  company_events,
  tickets,
  incidents,
  incident_responses
TO obertrack_mcp_ro;

-- Comprobación rápida (debe devolver 'on' y '15s'):
--   SELECT rolname, rolconfig FROM pg_roles WHERE rolname = 'obertrack_mcp_ro';
