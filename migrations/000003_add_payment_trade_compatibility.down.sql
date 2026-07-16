ALTER TABLE payments
    DROP INDEX idx_payments_trade_status,
    DROP INDEX uk_payments_active_trade,
    DROP COLUMN active_trade_id,
    DROP COLUMN trade_id;
