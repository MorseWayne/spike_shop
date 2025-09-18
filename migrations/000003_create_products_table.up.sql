-- 商品表迁移
-- 支持商品基本信息管理和检索

CREATE TABLE IF NOT EXISTS `products` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '商品ID',
  `name` varchar(255) NOT NULL COMMENT '商品名称',
  `description` text COMMENT '商品描述',
  `price` decimal(10,2) NOT NULL COMMENT '商品价格',
  `category_id` bigint unsigned COMMENT '商品分类ID',
  `brand` varchar(100) COMMENT '品牌',
  `sku` varchar(100) NOT NULL COMMENT '商品SKU，唯一',
  `status` enum('active', 'inactive', 'deleted') NOT NULL DEFAULT 'active' COMMENT '商品状态',
  `weight` decimal(8,3) COMMENT '商品重量(kg)',
  `image_url` varchar(500) COMMENT '商品图片URL',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sku` (`sku`),
  KEY `idx_name` (`name`),
  KEY `idx_category_id` (`category_id`),
  KEY `idx_status` (`status`),
  KEY `idx_price` (`price`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='商品表';
