-- Вставка базовых ролей
INSERT INTO roles (name, description) VALUES
    ('user', 'Обычный пользователь'),
    ('manager', 'Менеджер с расширенными правами'),
    ('admin', 'Администратор с полными правами')
ON CONFLICT (name) DO NOTHING;

-- Вставка разрешений
INSERT INTO permissions (code, description) VALUES
    ('product.create', 'Создание продуктов'),
    ('product.read', 'Просмотр продуктов'),
    ('product.update', 'Обновление продуктов'),
    ('product.delete', 'Удаление продуктов'),
    ('user.create', 'Создание пользователей'),
    ('user.read', 'Просмотр пользователей'),
    ('user.update', 'Обновление пользователей'),
    ('user.delete', 'Удаление пользователей'),
    ('order.create', 'Создание заказов'),
    ('order.read', 'Просмотр заказов'),
    ('order.update', 'Обновление заказов'),
    ('order.delete', 'Удаление заказов')
ON CONFLICT (code) DO NOTHING;

-- Назначение разрешений роли 'user'
INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'user' AND p.code IN (
    'product.read',
    'order.create',
    'order.read'
)
ON CONFLICT DO NOTHING;

-- Назначение разрешений роли 'manager'
INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'manager' AND p.code IN (
    'product.create',
    'product.read',
    'product.update',
    'user.read',
    'order.create',
    'order.read',
    'order.update'
    )
ON CONFLICT DO NOTHING;

-- Назначение всех разрешений роли 'admin'
INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;