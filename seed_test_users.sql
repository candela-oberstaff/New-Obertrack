-- Seed de datos de prueba (local). Password de todos: Obertrack2026!
BEGIN;

-- Profesionales y Customer Success de Acme S.A (empleador id 2)
INSERT INTO users (name, email, password, user_type, is_active, empleador_id, job_title, email_verified_at, created_at, updated_at) VALUES
('Laura Méndez',   'laura.mendez@acme.com',      '$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'profesional',      true, 2, 'Diseñadora UX',          NOW(), NOW(), NOW()),
('Diego Ramírez',  'diego.ramirez@acme.com',     '$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'profesional',      true, 2, 'Desarrollador Backend',  NOW(), NOW(), NOW()),
('Valentina Ríos', 'valentina.rios@acme.com',    '$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'profesional',      true, 2, 'Analista QA',            NOW(), NOW(), NOW()),
('Carmen Soto',    'carmen.soto@oberstaff.com',  '$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'customer_success', true, 2, 'Customer Success',       NOW(), NOW(), NOW()),
('Pedro Aguilar',  'pedro.aguilar@oberstaff.com','$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'customer_success', true, 2, 'Customer Success',       NOW(), NOW(), NOW());

-- Nueva empresa: Globex Corp (empleador = responsable de la empresa)
INSERT INTO users (name, email, password, user_type, is_active, company_name, industry, job_title, email_verified_at, created_at, updated_at) VALUES
('Gabriela Torres', 'gabriela@globex.com', '$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'empleador', true, 'Globex Corp', 'Tecnología', 'CEO', NOW(), NOW(), NOW());

-- Usuarios de Globex Corp
INSERT INTO users (name, email, password, user_type, is_active, empleador_id, job_title, email_verified_at, created_at, updated_at) VALUES
('Andrés Vega',     'andres.vega@globex.com',     '$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'profesional',      true, (SELECT id FROM users WHERE email = 'gabriela@globex.com'), 'Desarrollador Frontend',    NOW(), NOW(), NOW()),
('Sofía Navarro',   'sofia.navarro@globex.com',   '$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'profesional',      true, (SELECT id FROM users WHERE email = 'gabriela@globex.com'), 'Especialista en Marketing', NOW(), NOW(), NOW()),
('Mateo Ortiz',     'mateo.ortiz@globex.com',     '$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'profesional',      true, (SELECT id FROM users WHERE email = 'gabriela@globex.com'), 'Soporte TI',                NOW(), NOW(), NOW()),
('Lucía Fernández', 'lucia.fernandez@globex.com', '$2a$10$CNCWzx4hvdADmTz79Z45LuQ1QlZy/PzMWMXVXdt6VhriUHWDob8b6', 'customer_success', true, (SELECT id FROM users WHERE email = 'gabriela@globex.com'), 'Customer Success',          NOW(), NOW(), NOW());

COMMIT;
