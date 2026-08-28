CREATE TABLE restaurant_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    url TEXT NOT NULL CHECK (length(trim(url)) > 0),
    position INTEGER NOT NULL CHECK (position >= 0),
    is_cover BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (restaurant_id, position),
    UNIQUE (restaurant_id, url)
);

CREATE UNIQUE INDEX idx_restaurant_images_one_cover
    ON restaurant_images (restaurant_id)
    WHERE is_cover;

ALTER TABLE menu_items
    ADD COLUMN image_url TEXT CHECK (image_url IS NULL OR length(trim(image_url)) > 0);

INSERT INTO restaurant_images (id, restaurant_id, url, position, is_cover) VALUES
    (
        '60000000-0000-0000-0000-000000000001',
        '10000000-0000-0000-0000-000000000001',
        'https://images.unsplash.com/photo-1579751626657-72bc17010498?auto=format&fit=crop&w=1200&q=80',
        0,
        true
    ),
    (
        '60000000-0000-0000-0000-000000000002',
        '10000000-0000-0000-0000-000000000001',
        'https://images.unsplash.com/photo-1513104890138-7c749659a591?auto=format&fit=crop&w=1200&q=80',
        1,
        false
    );

UPDATE menu_items
SET image_url = CASE id
    WHEN '30000000-0000-0000-0000-000000000001' THEN
        'https://images.unsplash.com/photo-1574071318508-1cdbab80d002?auto=format&fit=crop&w=800&q=80'
    WHEN '30000000-0000-0000-0000-000000000002' THEN
        'https://images.unsplash.com/photo-1628840042765-356cda07504e?auto=format&fit=crop&w=800&q=80'
    ELSE image_url
END
WHERE id IN (
    '30000000-0000-0000-0000-000000000001',
    '30000000-0000-0000-0000-000000000002'
);

INSERT INTO restaurants (
    id, external_restaurant_id, name, description, status, integration_url
) VALUES (
    '10000000-0000-0000-0000-000000000002',
    'photo-free-cafe',
    'Photo-free Cafe',
    'Демонстрационное заведение без фотографий',
    'temporarily_unavailable',
    'http://restaurant:8081/api/v1'
);
