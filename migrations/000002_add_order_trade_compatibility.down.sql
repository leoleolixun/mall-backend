ALTER TABLE orders
    DROP INDEX idx_orders_trade_status_id,
    DROP INDEX uk_orders_trade_merchant,
    DROP COLUMN settlement_amount,
    DROP COLUMN commission_amount,
    DROP COLUMN commission_rate_bps,
    DROP COLUMN merchant_name,
    DROP COLUMN trade_id;
