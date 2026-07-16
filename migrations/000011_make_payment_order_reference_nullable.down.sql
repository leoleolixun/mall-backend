SET @go_mall_sql = (
    SELECT IF(
        COUNT(*) = 1,
        'ALTER TABLE payments MODIFY COLUMN order_id BIGINT NOT NULL',
        'SELECT 1'
    )
    FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'payments' AND column_name = 'order_id'
);
PREPARE go_mall_stmt FROM @go_mall_sql;
EXECUTE go_mall_stmt;
DEALLOCATE PREPARE go_mall_stmt;

SET @go_mall_sql = (
    SELECT IF(
        COUNT(*) = 1,
        'ALTER TABLE payments MODIFY COLUMN order_no VARCHAR(64) NOT NULL',
        'SELECT 1'
    )
    FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'payments' AND column_name = 'order_no'
);
PREPARE go_mall_stmt FROM @go_mall_sql;
EXECUTE go_mall_stmt;
DEALLOCATE PREPARE go_mall_stmt;

SET @go_mall_sql = (
    SELECT IF(
        COUNT(*) = 1,
        'ALTER TABLE payments MODIFY COLUMN merchant_id BIGINT NOT NULL',
        'SELECT 1'
    )
    FROM information_schema.columns
    WHERE table_schema = DATABASE() AND table_name = 'payments' AND column_name = 'merchant_id'
);
PREPARE go_mall_stmt FROM @go_mall_sql;
EXECUTE go_mall_stmt;
DEALLOCATE PREPARE go_mall_stmt;
