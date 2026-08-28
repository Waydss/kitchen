ALTER TABLE orders ADD COLUMN rejection_reason TEXT;

UPDATE orders
SET rejection_reason = 'restaurant_rejected'
WHERE status = 'rejected';

ALTER TABLE orders ADD CONSTRAINT orders_rejection_reason_matches_status CHECK (
    (status = 'rejected' AND rejection_reason IS NOT NULL AND length(trim(rejection_reason)) > 0)
    OR (status <> 'rejected' AND rejection_reason IS NULL)
);
