ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_rejection_reason_matches_status;
ALTER TABLE orders DROP COLUMN IF EXISTS rejection_reason;
