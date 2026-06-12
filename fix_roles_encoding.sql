-- Corrige las descripciones que llegaron con la codificación rota
-- (el SQL original se pipeó por la consola de Windows y mangló los acentos).

UPDATE roles SET description = 'Operación diaria: gestiona sus tareas, registra horas y participa en el chat.'
WHERE name = 'Colaborador' AND deleted_at IS NULL;

UPDATE roles SET description = 'Coordina al equipo: todo lo del colaborador más visibilidad de reportes. Para aprobar horas, combinar con el flag de manager.'
WHERE name = 'Supervisor' AND deleted_at IS NULL;

UPDATE roles SET description = 'Auditoría / consulta: ve tareas, horas y chat sin poder modificar nada.'
WHERE name = 'Solo lectura' AND deleted_at IS NULL;

UPDATE roles SET description = 'Customer success asignado a la empresa: gestiona tickets y chat, consulta tareas.'
WHERE name = 'Soporte' AND deleted_at IS NULL;

SELECT name, description FROM roles WHERE deleted_at IS NULL ORDER BY tenant_id, name;
