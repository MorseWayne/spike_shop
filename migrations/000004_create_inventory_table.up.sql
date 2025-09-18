-- 库存表迁移
-- 支持商品库存管理和查询，优化并发读写性能

CREATE TABLE IF NOT EXISTS `inventory` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '库存ID',
  `product_id` bigint unsigned NOT NULL COMMENT '商品ID',
  `stock` int unsigned NOT NULL DEFAULT 0 COMMENT '当前库存数量',
  `reserved_stock` int unsigned NOT NULL DEFAULT 0 COMMENT '预留库存数量(购物车/未支付订单)',
  `sold_stock` int unsigned NOT NULL DEFAULT 0 COMMENT '已售库存数量',
  `reorder_point` int unsigned NOT NULL DEFAULT 10 COMMENT '补货提醒点',
  `max_stock` int unsigned NOT NULL DEFAULT 10000 COMMENT '最大库存限制',
  `version` int unsigned NOT NULL DEFAULT 0 COMMENT '乐观锁版本号',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_product_id` (`product_id`),
  KEY `idx_stock` (`stock`),
  KEY `idx_reorder_point` (`reorder_point`),
  KEY `idx_updated_at` (`updated_at`),
  CONSTRAINT `fk_inventory_product_id` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='库存表';

