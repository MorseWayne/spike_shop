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

-- 插入一些测试数据
INSERT IGNORE INTO `products` (`name`, `description`, `price`, `sku`, `brand`, `status`) VALUES 
('iPhone 15 Pro', '苹果最新款旗舰手机，配备A17 Pro芯片', 8999.00, 'IPHONE-15-PRO-128GB', 'Apple', 'active'),
('小米14', '小米14系列智能手机，骁龙8 Gen3处理器', 3999.00, 'MI-14-256GB-BLACK', '小米', 'active'),
('MacBook Air M3', '全新M3芯片MacBook Air，轻薄便携', 8999.00, 'MBA-M3-13-256GB', 'Apple', 'active'),
('索尼WH-1000XM5', '索尼旗舰降噪耳机，顶级音质体验', 2399.00, 'SONY-WH-1000XM5-BLACK', 'Sony', 'active'),
('Nintendo Switch OLED', '任天堂Switch OLED版游戏主机', 2399.00, 'NSW-OLED-WHITE', 'Nintendo', 'active');
