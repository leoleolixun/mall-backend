ALTER TABLE orders
    ADD COLUMN trade_id BIGINT NULL AFTER id,
    ADD COLUMN merchant_name VARCHAR(100) NULL AFTER merchant_id,
    ADD COLUMN commission_rate_bps INT NULL AFTER payable_amount,
    ADD COLUMN commission_amount BIGINT NULL AFTER commission_rate_bps,
    ADD COLUMN settlement_amount BIGINT NULL AFTER commission_amount,
    ADD UNIQUE KEY uk_orders_trade_merchant (trade_id, merchant_id),
    ADD KEY idx_orders_trade_status_id (trade_id, status, id);
