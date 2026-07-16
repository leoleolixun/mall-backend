ALTER TABLE payments
    ADD COLUMN trade_id BIGINT NULL AFTER id,
    ADD COLUMN active_trade_id BIGINT NULL AFTER trade_id,
    ADD UNIQUE KEY uk_payments_active_trade (active_trade_id),
    ADD KEY idx_payments_trade_status (trade_id, status);
