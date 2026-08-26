CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE restaurants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_restaurant_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('open', 'temporarily_unavailable', 'disabled')),
    integration_url TEXT NOT NULL CHECK (length(trim(integration_url)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_restaurants_visible ON restaurants (name, id)
    WHERE status <> 'disabled';

CREATE TABLE menu_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (restaurant_id, name),
    UNIQUE (id, restaurant_id)
);

CREATE INDEX idx_menu_categories_restaurant
    ON menu_categories (restaurant_id, position, id);

CREATE TABLE menu_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    category_id UUID NOT NULL,
    external_item_id TEXT NOT NULL CHECK (length(trim(external_item_id)) > 0),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    description TEXT NOT NULL DEFAULT '',
    price BIGINT NOT NULL CHECK (price > 0),
    is_available BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (category_id, restaurant_id)
        REFERENCES menu_categories(id, restaurant_id),
    UNIQUE (restaurant_id, external_item_id),
    UNIQUE (id, restaurant_id)
);

CREATE INDEX idx_menu_items_catalog
    ON menu_items (restaurant_id, category_id, is_available, name, id);

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL,
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    status TEXT NOT NULL CHECK (status IN (
        'restaurant_confirmation_pending', 'confirmed', 'rejected',
        'preparing', 'ready', 'delivering', 'delivered', 'cancelled'
    )),
    total_price BIGINT NOT NULL CHECK (total_price > 0),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 8 AND 128),
    external_order_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (customer_id, idempotency_key),
    UNIQUE (id, restaurant_id)
);

CREATE UNIQUE INDEX idx_orders_restaurant_external_order
    ON orders (restaurant_id, external_order_id)
    WHERE external_order_id IS NOT NULL;
CREATE INDEX idx_orders_customer_created
    ON orders (customer_id, created_at DESC, id);
CREATE INDEX idx_orders_pending
    ON orders (created_at, id)
    WHERE status = 'restaurant_confirmation_pending';

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL,
    restaurant_id UUID NOT NULL,
    menu_item_id UUID NOT NULL,
    external_item_id TEXT NOT NULL,
    name TEXT NOT NULL,
    unit_price BIGINT NOT NULL CHECK (unit_price > 0),
    quantity INTEGER NOT NULL CHECK (quantity BETWEEN 1 AND 20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (order_id, restaurant_id) REFERENCES orders(id, restaurant_id) ON DELETE CASCADE,
    FOREIGN KEY (menu_item_id, restaurant_id) REFERENCES menu_items(id, restaurant_id),
    UNIQUE (order_id, menu_item_id)
);

CREATE INDEX idx_order_items_order ON order_items (order_id, id);

CREATE TABLE order_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    previous_status TEXT,
    status TEXT NOT NULL CHECK (status IN (
        'restaurant_confirmation_pending', 'confirmed', 'rejected',
        'preparing', 'ready', 'delivering', 'delivered', 'cancelled'
    )),
    source TEXT NOT NULL CHECK (source IN ('customer', 'restaurant', 'system')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_status_history_order
    ON order_status_history (order_id, created_at, id);
