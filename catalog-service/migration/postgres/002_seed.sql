-- Вставка тестовых категорий
INSERT INTO categories (id, name, created_at) VALUES
    ('550e8400-e29b-41d4-a716-446655440001', 'Electronics', NOW()),
    ('550e8400-e29b-41d4-a716-446655440002', 'Books', NOW()),
    ('550e8400-e29b-41d4-a716-446655440003', 'Clothing', NOW()),
    ('550e8400-e29b-41d4-a716-446655440004', 'Home & Garden', NOW()),
    ('550e8400-e29b-41d4-a716-446655440005', 'Sports & Outdoors', NOW())
ON CONFLICT (id) DO NOTHING;

-- Вставка тестовых товаров
INSERT INTO products (id, name, description, price, category_id, created_at) VALUES
    (
        '650e8400-e29b-41d4-a716-446655440001',
        'Laptop Dell XPS 15',
        'High-performance laptop with Intel Core i7, 16GB RAM, 512GB SSD. Perfect for professionals and creators.',
        1299.99,
        '550e8400-e29b-41d4-a716-446655440001',
        NOW()
    ),
    (
        '650e8400-e29b-41d4-a716-446655440002',
        'Wireless Mouse Logitech MX Master 3',
        'Ergonomic wireless mouse with precision tracking and customizable buttons.',
        99.99,
        '550e8400-e29b-41d4-a716-446655440001',
        NOW()
    ),
    (
        '650e8400-e29b-41d4-a716-446655440003',
        'Clean Code by Robert Martin',
        'A handbook of agile software craftsmanship. Essential reading for every developer.',
        42.99,
        '550e8400-e29b-41d4-a716-446655440002',
        NOW()
    ),
    (
        '650e8400-e29b-41d4-a716-446655440004',
        'The Pragmatic Programmer',
        'Your journey to mastery. Classic book about software development best practices.',
        49.99,
        '550e8400-e29b-41d4-a716-446655440002',
        NOW()
    ),
    (
        '650e8400-e29b-41d4-a716-446655440005',
        'Cotton T-Shirt',
        'Comfortable 100% cotton t-shirt available in multiple colors and sizes.',
        19.99,
        '550e8400-e29b-41d4-a716-446655440003',
        NOW()
    ),
    (
        '650e8400-e29b-41d4-a716-446655440006',
        'Jeans Levi''s 501',
        'Classic straight fit jeans made from premium denim.',
        89.99,
        '550e8400-e29b-41d4-a716-446655440003',
        NOW()
    ),
    (
        '650e8400-e29b-41d4-a716-446655440007',
        'Garden Tool Set',
        'Complete 10-piece gardening tool set with carrying case.',
        79.99,
        '550e8400-e29b-41d4-a716-446655440004',
        NOW()
    ),
    (
        '650e8400-e29b-41d4-a716-446655440008',
        'Yoga Mat Premium',
        'Non-slip yoga mat with extra cushioning for comfort during practice.',
        34.99,
        '550e8400-e29b-41d4-a716-446655440005',
        NOW()
    )
ON CONFLICT (id) DO NOTHING;