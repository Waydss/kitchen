CREATE FUNCTION ensure_restaurant_images_have_cover()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    restaurant_ids UUID[];
    restaurant_id UUID;
BEGIN
    IF TG_OP = 'INSERT' THEN
        restaurant_ids := ARRAY[NEW.restaurant_id];
    ELSIF TG_OP = 'DELETE' THEN
        restaurant_ids := ARRAY[OLD.restaurant_id];
    ELSE
        restaurant_ids := ARRAY[OLD.restaurant_id, NEW.restaurant_id];
    END IF;

    FOREACH restaurant_id IN ARRAY restaurant_ids LOOP
        IF EXISTS (
            SELECT 1 FROM restaurant_images WHERE restaurant_images.restaurant_id = restaurant_id
        ) AND NOT EXISTS (
            SELECT 1
            FROM restaurant_images
            WHERE restaurant_images.restaurant_id = restaurant_id AND is_cover
        ) THEN
            RAISE EXCEPTION 'restaurant % has images but no cover', restaurant_id
                USING ERRCODE = 'check_violation';
        END IF;
    END LOOP;
    RETURN NULL;
END;
$$;

CREATE CONSTRAINT TRIGGER restaurant_images_require_cover
AFTER INSERT OR UPDATE OR DELETE ON restaurant_images
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION ensure_restaurant_images_have_cover();
