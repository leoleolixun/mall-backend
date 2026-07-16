ALTER TABLE refunds
    ADD COLUMN trade_id BIGINT NULL AFTER id,
    ADD COLUMN payment_allocation_id BIGINT NULL AFTER payment_id,
    ADD KEY idx_refunds_trade_status (trade_id, status),
    ADD KEY idx_refunds_payment_allocation (payment_allocation_id);
