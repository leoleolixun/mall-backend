ALTER TABLE categories
    ADD KEY idx_categories_merchant_status_deleted_sort_id (merchant_id, status, deleted_at, sort, id);
