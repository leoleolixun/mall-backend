ALTER TABLE merchants
    DROP CHECK chk_merchants_commission_rate_bps,
    DROP COLUMN commission_rate_bps;
