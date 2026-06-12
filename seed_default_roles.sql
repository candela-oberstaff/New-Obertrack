-- Roles preconfigurados por empresa (módulo Roles y Grupos).
-- Permisos: {"módulo": "none"|"view"|"edit"} sobre tasks, hours, reports, chat, tickets, tutorials.
-- Idempotente: no duplica si el rol ya existe en la empresa.

WITH presets (name, description, permissions) AS (
  VALUES
    ('Colaborador',
     'Operación diaria: gestiona sus tareas, registra horas y participa en el chat.',
     '{"tasks":"edit","hours":"edit","chat":"edit","tutorials":"view","reports":"none","tickets":"none"}'),
    ('Supervisor',
     'Coordina al equipo: todo lo del colaborador más visibilidad de reportes. Para aprobar horas, combinar con el flag de manager.',
     '{"tasks":"edit","hours":"edit","chat":"edit","tutorials":"view","reports":"view","tickets":"none"}'),
    ('Solo lectura',
     'Auditoría / consulta: ve tareas, horas y chat sin poder modificar nada.',
     '{"tasks":"view","hours":"view","chat":"view","tutorials":"view","reports":"view","tickets":"none"}'),
    ('Soporte',
     'Customer success asignado a la empresa: gestiona tickets y chat, consulta tareas.',
     '{"tasks":"view","hours":"none","chat":"edit","tutorials":"view","reports":"none","tickets":"edit"}')
),
tenants AS (
  SELECT id AS tenant_id FROM users
  WHERE user_type = 'empleador' AND deleted_at IS NULL
)
INSERT INTO roles (tenant_id, name, description, permissions, created_by, created_at, updated_at)
SELECT t.tenant_id, p.name, p.description, p.permissions, 1, NOW(), NOW()
FROM tenants t
CROSS JOIN presets p
WHERE NOT EXISTS (
  SELECT 1 FROM roles r
  WHERE r.tenant_id = t.tenant_id AND r.name = p.name AND r.deleted_at IS NULL
);

SELECT r.id, u.company_name AS empresa, r.name, r.permissions
FROM roles r JOIN users u ON u.id = r.tenant_id
WHERE r.deleted_at IS NULL
ORDER BY u.company_name, r.name;
