ALTER TABLE refunds
    DROP INDEX idx_refunds_payment_allocation,
    DROP INDEX idx_refunds_trade_status,
    DROP COLUMN payment_allocation_id,
    DROP COLUMN trade_id;
