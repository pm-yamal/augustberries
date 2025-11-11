-- Создание таблицы заказов
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    total_price DECIMAL(10, 2) NOT NULL,
    delivery_price DECIMAL(10, 2) NOT NULL DEFAULT 0,
    currency VARCHAR(10) NOT NULL DEFAULT 'RUB',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    -- Индексы для оптимизации запросов
    CONSTRAINT chk_total_price CHECK (total_price >= 0),
    CONSTRAINT chk_delivery_price CHECK (delivery_price >= 0),
    CONSTRAINT chk_currency CHECK (currency IN ('USD', 'EUR', 'RUB')),
    CONSTRAINT chk_status CHECK (status IN ('pending', 'confirmed', 'shipped', 'delivered', 'cancelled'))
);

-- Создание индексов для оптимизации запросов по заказам
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);

-- Создание таблицы позиций заказов
CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID NOT NULL,
    quantity INT NOT NULL,
    unit_price DECIMAL(10, 2) NOT NULL,
    
    -- Ограничения
    CONSTRAINT chk_quantity CHECK (quantity > 0),
    CONSTRAINT chk_unit_price CHECK (unit_price >= 0)
);

-- Создание индексов для оптимизации запросов по позициям заказов
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);
