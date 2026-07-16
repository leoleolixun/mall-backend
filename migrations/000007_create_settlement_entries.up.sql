CREATE TABLE settlement_entries (
    id BIGINT NOT NULL AUTO_INCREMENT,
    entry_no VARCHAR(64) NOT NULL,
    merchant_id BIGINT NOT NULL,
    order_id BIGINT NULL,
    refund_id BIGINT NULL,
    entry_type VARCHAR(32) NOT NULL,
    amount BIGINT NOT NULL,
    available_at DATETIME(3) NOT NULL,
    settlement_id BIGINT NULL,
    created_at DATETIME(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_settlement_entries_no (entry_no),
    KEY idx_settlement_entries_merchant_available_id (merchant_id, available_at, id),
    KEY idx_settlement_entries_order (order_id),
    KEY idx_settlement_entries_refund (refund_id),
    KEY idx_settlement_entries_settlement (settlement_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
