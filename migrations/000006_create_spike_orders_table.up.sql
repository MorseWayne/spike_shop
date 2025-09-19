-- 秒杀订单表迁移
-- 支持秒杀订单管理和去重约束

CREATE TABLE IF NOT EXISTS `spike_orders` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '秒杀订单ID',
  `spike_event_id` bigint unsigned NOT NULL COMMENT '秒杀活动ID',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `order_id` bigint unsigned COMMENT '关联的普通订单ID(异步创建)',
  `quantity` int unsigned NOT NULL DEFAULT 1 COMMENT '购买数量',
  `spike_price` decimal(10,2) NOT NULL COMMENT '成交价格',
  `total_amount` decimal(10,2) NOT NULL COMMENT '总金额',
  `status` enum('pending', 'paid', 'cancelled', 'expired') NOT NULL DEFAULT 'pending' COMMENT '订单状态',
  `idempotency_key` varchar(64) COMMENT '幂等键',
  `expire_at` timestamp NULL COMMENT '订单过期时间',
  `paid_at` timestamp NULL COMMENT '支付完成时间',
  `cancelled_at` timestamp NULL COMMENT '取消时间',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_spike_event` (`user_id`, `spike_event_id`) COMMENT '用户活动去重约束',
  UNIQUE KEY `uk_idempotency_key` (`idempotency_key`) COMMENT '幂等键唯一约束',
  KEY `idx_spike_event_id` (`spike_event_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_order_id` (`order_id`),
  KEY `idx_status` (`status`),
  KEY `idx_expire_at` (`expire_at`),
  KEY `idx_created_at` (`created_at`),
  CONSTRAINT `fk_spike_orders_spike_event_id` FOREIGN KEY (`spike_event_id`) REFERENCES `spike_events` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_spike_orders_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `chk_quantity_positive` CHECK (`quantity` > 0),
  CONSTRAINT `chk_spike_price_positive` CHECK (`spike_price` > 0),
  CONSTRAINT `chk_total_amount_positive` CHECK (`total_amount` > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='秒杀订单表';
