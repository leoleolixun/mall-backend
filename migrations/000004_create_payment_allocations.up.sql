CREATE TABLE payment_allocations (
    id BIGINT NOT NULL AUTO_INCREMENT,
    payment_id BIGINT NOT NULL,
    trade_id BIGINT NOT NULL,
    order_id BIGINT NOT NULL,
    merchant_id BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    refunded_amount BIGINT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_payment_allocations_payment_order (payment_id, order_id),
    KEY idx_payment_allocations_trade (trade_id),
    KEY idx_payment_allocations_order (order_id),
    KEY idx_payment_allocations_merchant_created_id (merchant_id, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
