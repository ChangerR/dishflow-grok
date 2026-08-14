-- +goose Up is not used; golang-migrate format.

CREATE TABLE stores (
  id CHAR(26) NOT NULL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  phone VARCHAR(32) NULL,
  address VARCHAR(255) NULL,
  business_hours VARCHAR(32) NOT NULL DEFAULT '09:00-21:00',
  pickup_minutes INT NOT NULL DEFAULT 15,
  scheduled_pickup_enabled TINYINT(1) NOT NULL DEFAULT 1,
  pickup_advance_days INT NOT NULL DEFAULT 7,
  pickup_slot_minutes INT NOT NULL DEFAULT 15,
  pickup_slot_capacity INT NOT NULL DEFAULT 5,
  pickup_min_lead_minutes INT NOT NULL DEFAULT 30,
  announcement TEXT NULL,
  is_open TINYINT(1) NOT NULL DEFAULT 1,
  timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  brand_name VARCHAR(128) NULL,
  theme_color VARCHAR(16) NULL,
  logo_url VARCHAR(512) NULL,
  privacy_policy TEXT NULL,
  refund_policy TEXT NULL,
  qualifications TEXT NULL,
  wechat_appid VARCHAR(64) NULL,
  wechat_appsecret_enc TEXT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_stores_wechat_appid (wechat_appid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE admin_users (
  id CHAR(26) NOT NULL PRIMARY KEY,
  login_name VARCHAR(64) NOT NULL,
  display_name VARCHAR(64) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  is_platform_admin TINYINT(1) NOT NULL DEFAULT 0,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_admin_users_login (login_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE admin_sessions (
  id CHAR(26) NOT NULL PRIMARY KEY,
  admin_user_id CHAR(26) NOT NULL,
  token_hash CHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  last_seen_at DATETIME(3) NOT NULL,
  last_renewed_at DATETIME(3) NOT NULL,
  absolute_expires_at DATETIME(3) NOT NULL,
  idle_expires_at DATETIME(3) NOT NULL,
  revoked_at DATETIME(3) NULL,
  UNIQUE KEY uk_admin_sessions_token (token_hash),
  KEY idx_admin_sessions_user (admin_user_id),
  CONSTRAINT fk_admin_sessions_user FOREIGN KEY (admin_user_id) REFERENCES admin_users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE shop_members (
  store_id CHAR(26) NOT NULL,
  admin_user_id CHAR(26) NOT NULL,
  role VARCHAR(16) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (store_id, admin_user_id),
  UNIQUE KEY uk_shop_members_user (admin_user_id),
  CONSTRAINT fk_shop_members_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_shop_members_user FOREIGN KEY (admin_user_id) REFERENCES admin_users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE shop_applications (
  id CHAR(26) NOT NULL PRIMARY KEY,
  applicant_admin_user_id CHAR(26) NOT NULL,
  store_name VARCHAR(128) NOT NULL,
  contact VARCHAR(128) NOT NULL,
  status VARCHAR(16) NOT NULL,
  note VARCHAR(255) NULL,
  reviewed_at DATETIME(3) NULL,
  reviewer_id CHAR(26) NULL,
  created_at DATETIME(3) NOT NULL,
  KEY idx_shop_applications_applicant (applicant_admin_user_id),
  KEY idx_shop_applications_status (status),
  CONSTRAINT fk_shop_applications_applicant FOREIGN KEY (applicant_admin_user_id) REFERENCES admin_users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE shop_join_requests (
  id CHAR(26) NOT NULL PRIMARY KEY,
  applicant_admin_user_id CHAR(26) NOT NULL,
  store_id CHAR(26) NOT NULL,
  requested_role VARCHAR(16) NOT NULL,
  status VARCHAR(16) NOT NULL,
  note VARCHAR(255) NULL,
  reviewed_at DATETIME(3) NULL,
  reviewer_id CHAR(26) NULL,
  created_at DATETIME(3) NOT NULL,
  KEY idx_join_requests_store (store_id, status),
  CONSTRAINT fk_join_requests_applicant FOREIGN KEY (applicant_admin_user_id) REFERENCES admin_users(id),
  CONSTRAINT fk_join_requests_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE customers (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  wechat_openid VARCHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_customers_store_openid (store_id, wechat_openid),
  CONSTRAINT fk_customers_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE customer_sessions (
  id CHAR(26) NOT NULL PRIMARY KEY,
  customer_id CHAR(26) NOT NULL,
  store_id CHAR(26) NOT NULL,
  token_hash CHAR(64) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_customer_sessions_token (token_hash),
  KEY idx_customer_sessions_customer (customer_id),
  CONSTRAINT fk_customer_sessions_customer FOREIGN KEY (customer_id) REFERENCES customers(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE member_settings (
  store_id CHAR(26) NOT NULL PRIMARY KEY,
  points_per_yuan INT NOT NULL DEFAULT 1,
  newbie_coupon_template_id CHAR(26) NULL,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_member_settings_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE categories (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  name VARCHAR(64) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  deleted_at DATETIME(3) NULL,
  restore_until DATETIME(3) NULL,
  restore_closed TINYINT(1) NOT NULL DEFAULT 0,
  delete_batch_id CHAR(26) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_categories_store (store_id, deleted_at, sort_order),
  CONSTRAINT fk_categories_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE products (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  category_id CHAR(26) NOT NULL,
  code VARCHAR(64) NULL,
  name VARCHAR(128) NOT NULL,
  description TEXT NULL,
  image_url VARCHAR(512) NULL,
  sort_order INT NOT NULL DEFAULT 0,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  sold_out TINYINT(1) NOT NULL DEFAULT 0,
  packing_fee_cents BIGINT NOT NULL DEFAULT 0,
  deleted_at DATETIME(3) NULL,
  restore_until DATETIME(3) NULL,
  restore_closed TINYINT(1) NOT NULL DEFAULT 0,
  delete_batch_id CHAR(26) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_products_store (store_id, deleted_at, category_id),
  CONSTRAINT fk_products_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_products_category FOREIGN KEY (category_id) REFERENCES categories(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE skus (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  product_id CHAR(26) NOT NULL,
  name VARCHAR(64) NOT NULL,
  price_cents BIGINT NOT NULL,
  stock_mode VARCHAR(16) NOT NULL,
  daily_stock INT NOT NULL DEFAULT 0,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_skus_product (product_id, sort_order),
  CONSTRAINT fk_skus_product FOREIGN KEY (product_id) REFERENCES products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE option_groups (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  product_id CHAR(26) NOT NULL,
  name VARCHAR(64) NOT NULL,
  selection_type VARCHAR(16) NOT NULL,
  required TINYINT(1) NOT NULL DEFAULT 0,
  min_select INT NOT NULL DEFAULT 0,
  max_select INT NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_option_groups_product (product_id, sort_order),
  CONSTRAINT fk_option_groups_product FOREIGN KEY (product_id) REFERENCES products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE option_items (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  option_group_id CHAR(26) NOT NULL,
  name VARCHAR(64) NOT NULL,
  price_cents BIGINT NOT NULL DEFAULT 0,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_option_items_group (option_group_id, sort_order),
  CONSTRAINT fk_option_items_group FOREIGN KEY (option_group_id) REFERENCES option_groups(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE daily_inventory (
  store_id CHAR(26) NOT NULL,
  sku_id CHAR(26) NOT NULL,
  business_date DATE NOT NULL,
  available_qty INT NOT NULL,
  reserved_qty INT NOT NULL DEFAULT 0,
  sold_qty INT NOT NULL DEFAULT 0,
  PRIMARY KEY (store_id, sku_id, business_date),
  CONSTRAINT chk_inventory_nonneg CHECK (available_qty >= 0 AND reserved_qty >= 0 AND sold_qty >= 0),
  CONSTRAINT chk_inventory_balance CHECK (available_qty >= reserved_qty + sold_qty)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE inventory_movements (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  sku_id CHAR(26) NOT NULL,
  business_date DATE NOT NULL,
  delta INT NOT NULL,
  reason VARCHAR(64) NOT NULL,
  order_id CHAR(26) NULL,
  actor_id CHAR(26) NULL,
  note VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL,
  KEY idx_inv_movements_sku (store_id, sku_id, business_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE dining_tables (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  table_no VARCHAR(32) NOT NULL,
  area VARCHAR(64) NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  token_hash CHAR(64) NOT NULL,
  mp_code_object_key VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_tables_store_no (store_id, table_no),
  UNIQUE KEY uk_tables_token (token_hash),
  CONSTRAINT fk_tables_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE promotions (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  name VARCHAR(64) NOT NULL,
  threshold_cents BIGINT NOT NULL,
  discount_cents BIGINT NOT NULL,
  scope VARCHAR(32) NOT NULL DEFAULT 'ALL',
  stack_policy VARCHAR(32) NOT NULL DEFAULT 'BEST_OF',
  starts_at DATETIME(3) NOT NULL,
  ends_at DATETIME(3) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_promotions_store (store_id, enabled),
  CONSTRAINT fk_promotions_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE coupon_templates (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  name VARCHAR(64) NOT NULL,
  min_spend_cents BIGINT NOT NULL,
  discount_cents BIGINT NOT NULL,
  scope VARCHAR(32) NOT NULL DEFAULT 'ALL',
  starts_at DATETIME(3) NOT NULL,
  ends_at DATETIME(3) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  public_claim TINYINT(1) NOT NULL DEFAULT 0,
  audience VARCHAR(32) NOT NULL DEFAULT 'ALL',
  redeemable TINYINT(1) NOT NULL DEFAULT 0,
  points_cost INT NOT NULL DEFAULT 0,
  permanently_deleted TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_coupon_templates_store (store_id, enabled),
  CONSTRAINT fk_coupon_templates_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE customer_memberships (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  customer_id CHAR(26) NOT NULL,
  member_no VARCHAR(32) NOT NULL,
  phone_enc TEXT NOT NULL,
  phone_hash CHAR(64) NOT NULL,
  phone_last4 CHAR(4) NOT NULL,
  phone_country_code VARCHAR(8) NOT NULL DEFAULT '86',
  status VARCHAR(16) NOT NULL,
  points_balance INT NOT NULL DEFAULT 0,
  joined_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_membership_customer (store_id, customer_id),
  UNIQUE KEY uk_membership_no (store_id, member_no),
  UNIQUE KEY uk_membership_phone_hash (store_id, phone_hash),
  CONSTRAINT fk_membership_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_membership_customer FOREIGN KEY (customer_id) REFERENCES customers(id),
  CONSTRAINT chk_points_nonneg CHECK (points_balance >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE member_points_ledger (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  membership_id CHAR(26) NOT NULL,
  delta INT NOT NULL,
  balance_after INT NOT NULL,
  reason_type VARCHAR(32) NOT NULL,
  ref_id CHAR(26) NULL,
  note VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL,
  KEY idx_points_ledger_member (membership_id, created_at),
  CONSTRAINT fk_points_ledger_membership FOREIGN KEY (membership_id) REFERENCES customer_memberships(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE customer_coupons (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  customer_id CHAR(26) NOT NULL,
  template_id CHAR(26) NOT NULL,
  status VARCHAR(16) NOT NULL,
  claimed_at DATETIME(3) NOT NULL,
  used_at DATETIME(3) NULL,
  order_id CHAR(26) NULL,
  UNIQUE KEY uk_customer_coupon_once (customer_id, template_id),
  KEY idx_customer_coupons_customer (store_id, customer_id, status),
  CONSTRAINT fk_customer_coupons_template FOREIGN KEY (template_id) REFERENCES coupon_templates(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pickup_slot_capacity (
  store_id CHAR(26) NOT NULL,
  slot_start DATETIME(3) NOT NULL,
  capacity_snapshot INT NOT NULL,
  reserved_orders INT NOT NULL DEFAULT 0,
  PRIMARY KEY (store_id, slot_start),
  CONSTRAINT chk_slot_reserved CHECK (reserved_orders >= 0 AND reserved_orders <= capacity_snapshot)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE store_pickup_counters (
  store_id CHAR(26) NOT NULL,
  business_date DATE NOT NULL,
  last_number INT NOT NULL,
  PRIMARY KEY (store_id, business_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE orders (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  customer_id CHAR(26) NOT NULL,
  quote_id CHAR(26) NOT NULL,
  scene VARCHAR(16) NOT NULL,
  status VARCHAR(32) NOT NULL,
  payment_status VARCHAR(32) NOT NULL,
  refund_status VARCHAR(32) NULL,
  table_id CHAR(26) NULL,
  table_no VARCHAR(32) NULL,
  pickup_type VARCHAR(16) NOT NULL,
  scheduled_for DATETIME(3) NULL,
  pickup_business_date DATE NOT NULL,
  pickup_number VARCHAR(16) NULL,
  goods_cents BIGINT NOT NULL,
  packing_cents BIGINT NOT NULL,
  discount_cents BIGINT NOT NULL,
  payable_cents BIGINT NOT NULL,
  applied_promotion_id CHAR(26) NULL,
  applied_coupon_id CHAR(26) NULL,
  remark VARCHAR(100) NULL,
  version INT NOT NULL DEFAULT 1,
  pickup_capacity_released_at DATETIME(3) NULL,
  points_awarded INT NOT NULL DEFAULT 0,
  points_reversed TINYINT(1) NOT NULL DEFAULT 0,
  is_mock TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  paid_at DATETIME(3) NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_orders_quote (quote_id),
  UNIQUE KEY uk_orders_pickup_no (store_id, pickup_business_date, pickup_number),
  KEY idx_orders_store_status (store_id, status, created_at),
  KEY idx_orders_customer (store_id, customer_id, created_at),
  CONSTRAINT fk_orders_store FOREIGN KEY (store_id) REFERENCES stores(id),
  CONSTRAINT fk_orders_customer FOREIGN KEY (customer_id) REFERENCES customers(id),
  CONSTRAINT chk_order_money CHECK (goods_cents >= 0 AND packing_cents >= 0 AND discount_cents >= 0 AND payable_cents >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE order_items (
  id CHAR(26) NOT NULL PRIMARY KEY,
  order_id CHAR(26) NOT NULL,
  store_id CHAR(26) NOT NULL,
  product_id CHAR(26) NOT NULL,
  sku_id CHAR(26) NOT NULL,
  product_name VARCHAR(128) NOT NULL,
  sku_name VARCHAR(64) NOT NULL,
  options_json JSON NOT NULL,
  qty INT NOT NULL,
  unit_price_cents BIGINT NOT NULL,
  packing_fee_cents BIGINT NOT NULL,
  line_total_cents BIGINT NOT NULL,
  KEY idx_order_items_order (order_id),
  CONSTRAINT fk_order_items_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE order_events (
  id CHAR(26) NOT NULL PRIMARY KEY,
  order_id CHAR(26) NOT NULL,
  store_id CHAR(26) NOT NULL,
  from_status VARCHAR(32) NULL,
  to_status VARCHAR(32) NOT NULL,
  actor_type VARCHAR(16) NOT NULL,
  actor_id CHAR(26) NULL,
  note VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL,
  KEY idx_order_events_order (order_id, created_at),
  CONSTRAINT fk_order_events_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE payments (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  order_id CHAR(26) NOT NULL,
  status VARCHAR(32) NOT NULL,
  prepay_id VARCHAR(64) NULL,
  wechat_transaction_id VARCHAR(64) NULL,
  amount_cents BIGINT NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  is_mock TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_payments_order (order_id),
  CONSTRAINT fk_payments_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE refunds (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  order_id CHAR(26) NOT NULL,
  merchant_refund_no VARCHAR(64) NOT NULL,
  amount_cents BIGINT NOT NULL,
  reason VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL,
  source VARCHAR(32) NOT NULL,
  wechat_refund_id VARCHAR(64) NULL,
  is_mock TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_refunds_no (merchant_refund_no),
  UNIQUE KEY uk_refunds_active_order (order_id),
  CONSTRAINT fk_refunds_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE payment_configs (
  store_id CHAR(26) NOT NULL PRIMARY KEY,
  mch_id VARCHAR(32) NULL,
  serial_no VARCHAR(64) NULL,
  api_v3_key_enc TEXT NULL,
  private_key_enc TEXT NULL,
  pub_key_id VARCHAR(64) NULL,
  pub_key_pem_enc TEXT NULL,
  platform_cert_enc TEXT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'draft',
  mock_payment TINYINT(1) NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_payment_configs_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE print_configs (
  store_id CHAR(26) NOT NULL PRIMARY KEY,
  appid VARCHAR(64) NULL,
  appsecret_enc TEXT NULL,
  auto_print TINYINT(1) NOT NULL DEFAULT 0,
  mock_print TINYINT(1) NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'draft',
  updated_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_print_configs_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE cloud_printers (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  sn VARCHAR(64) NOT NULL,
  key_enc TEXT NOT NULL,
  name VARCHAR(64) NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  copies INT NOT NULL DEFAULT 1,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  online TINYINT(1) NOT NULL DEFAULT 0,
  note VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_printers_sn (store_id, sn),
  CONSTRAINT fk_printers_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE cloud_print_jobs (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  type VARCHAR(16) NOT NULL,
  status VARCHAR(16) NOT NULL,
  order_id CHAR(26) NULL,
  purchase_list_id CHAR(26) NULL,
  printer_id CHAR(26) NULL,
  shangpeng_id VARCHAR(64) NULL,
  attempts INT NOT NULL DEFAULT 0,
  error_message VARCHAR(512) NULL,
  content TEXT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_print_jobs_store (store_id, created_at),
  KEY idx_print_jobs_order (order_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE materials (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  name VARCHAR(64) NOT NULL,
  image_url VARCHAR(512) NULL,
  category VARCHAR(64) NULL,
  unit VARCHAR(16) NOT NULL,
  default_qty DECIMAL(10,2) NOT NULL DEFAULT 1.00,
  note VARCHAR(255) NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_materials_name (store_id, name),
  CONSTRAINT fk_materials_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE purchase_lists (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  list_no VARCHAR(32) NOT NULL,
  business_date DATE NOT NULL,
  title VARCHAR(128) NOT NULL,
  status VARCHAR(16) NOT NULL,
  total_amount_cents BIGINT NULL,
  version INT NOT NULL DEFAULT 1,
  print_count INT NOT NULL DEFAULT 0,
  created_by CHAR(26) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_purchase_lists_no (store_id, list_no),
  KEY idx_purchase_lists_store (store_id, business_date),
  CONSTRAINT fk_purchase_lists_store FOREIGN KEY (store_id) REFERENCES stores(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE purchase_list_items (
  id CHAR(26) NOT NULL PRIMARY KEY,
  list_id CHAR(26) NOT NULL,
  store_id CHAR(26) NOT NULL,
  material_id CHAR(26) NULL,
  name_snapshot VARCHAR(64) NOT NULL,
  unit_snapshot VARCHAR(16) NOT NULL,
  qty DECIMAL(10,2) NOT NULL,
  note VARCHAR(255) NULL,
  UNIQUE KEY uk_purchase_item_material (list_id, material_id),
  CONSTRAINT fk_purchase_items_list FOREIGN KEY (list_id) REFERENCES purchase_lists(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE purchase_list_events (
  id CHAR(26) NOT NULL PRIMARY KEY,
  list_id CHAR(26) NOT NULL,
  store_id CHAR(26) NOT NULL,
  from_status VARCHAR(16) NULL,
  to_status VARCHAR(16) NOT NULL,
  actor_id CHAR(26) NULL,
  note VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL,
  KEY idx_purchase_events_list (list_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE idempotency_keys (
  id CHAR(26) NOT NULL PRIMARY KEY,
  subject VARCHAR(80) NOT NULL,
  idem_key VARCHAR(128) NOT NULL,
  request_hash CHAR(64) NOT NULL,
  status_code INT NOT NULL,
  response_body MEDIUMTEXT NOT NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_idempotency (subject, idem_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE webhook_events (
  id CHAR(26) NOT NULL PRIMARY KEY,
  provider_event_id VARCHAR(128) NOT NULL,
  store_id CHAR(26) NOT NULL,
  kind VARCHAR(32) NOT NULL,
  processed_at DATETIME(3) NOT NULL,
  UNIQUE KEY uk_webhook_event (provider_event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE outbox (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  payload JSON NOT NULL,
  created_at DATETIME(3) NOT NULL,
  published_at DATETIME(3) NULL,
  KEY idx_outbox_unpublished (published_at, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE audit_logs (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NULL,
  actor_id CHAR(26) NULL,
  actor_type VARCHAR(16) NOT NULL,
  action VARCHAR(64) NOT NULL,
  resource VARCHAR(64) NOT NULL,
  summary VARCHAR(512) NOT NULL,
  request_id VARCHAR(64) NULL,
  created_at DATETIME(3) NOT NULL,
  KEY idx_audit_store (store_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE exceptions (
  id CHAR(26) NOT NULL PRIMARY KEY,
  store_id CHAR(26) NOT NULL,
  biz_no VARCHAR(64) NOT NULL,
  error_type VARCHAR(32) NOT NULL,
  status VARCHAR(16) NOT NULL,
  message VARCHAR(512) NOT NULL,
  retries INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  KEY idx_exceptions_store (store_id, status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
