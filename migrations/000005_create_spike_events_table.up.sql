-- 秒杀活动表迁移
-- 支持秒杀活动管理和时间范围查询

CREATE TABLE IF NOT EXISTS `spike_events` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '秒杀活动ID',
  `product_id` bigint unsigned NOT NULL COMMENT '商品ID',
  `name` varchar(255) NOT NULL COMMENT '活动名称',
  `description` text COMMENT '活动描述',
  `spike_price` decimal(10,2) NOT NULL COMMENT '秒杀价格',
  `original_price` decimal(10,2) NOT NULL COMMENT '原价',
  `spike_stock` int unsigned NOT NULL COMMENT '秒杀库存数量',
  `sold_count` int unsigned NOT NULL DEFAULT 0 COMMENT '已售数量',
  `start_at` timestamp NOT NULL COMMENT '活动开始时间',
  `end_at` timestamp NOT NULL COMMENT '活动结束时间',
  `status` enum('pending', 'active', 'ended', 'cancelled') NOT NULL DEFAULT 'pending' COMMENT '活动状态',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_product_id` (`product_id`),
  KEY `idx_time_range` (`start_at`, `end_at`),
  KEY `idx_status` (`status`),
  KEY `idx_product_status_time` (`product_id`, `status`, `start_at`, `end_at`),
  KEY `idx_created_at` (`created_at`),
  CONSTRAINT `fk_spike_events_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE CASCADE,
  CONSTRAINT `chk_spike_price_positive` CHECK (`spike_price` > 0),
  CONSTRAINT `chk_original_price_positive` CHECK (`original_price` > 0),
  CONSTRAINT `chk_spike_stock_positive` CHECK (`spike_stock` > 0),
  CONSTRAINT `chk_time_range_valid` CHECK (`start_at` < `end_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='秒杀活动表';
