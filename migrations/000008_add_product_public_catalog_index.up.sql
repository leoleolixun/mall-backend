ALTER TABLE products
    ADD KEY idx_products_merchant_status_deleted_id (merchant_id, status, deleted_at, id);
