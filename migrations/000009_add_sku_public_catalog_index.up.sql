ALTER TABLE product_skus
    ADD KEY idx_skus_merchant_product_status_deleted (merchant_id, product_id, status, deleted_at);
