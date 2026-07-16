ALTER TABLE merchants
    ADD COLUMN commission_rate_bps INT NOT NULL DEFAULT 0,
    ADD CONSTRAINT chk_merchants_commission_rate_bps
        CHECK (commission_rate_bps >= 0 AND commission_rate_bps <= 10000);
