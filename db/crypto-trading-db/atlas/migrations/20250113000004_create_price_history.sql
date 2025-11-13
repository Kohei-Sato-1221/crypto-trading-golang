-- Create price_histories table
CREATE TABLE `price_histories` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `datetime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `product_code` varchar(50) NOT NULL,
  `price` float NOT NULL,
  `price_ratio_24h` float DEFAULT NULL COMMENT '24時間前との価格比率（少数形式: 例: 0.95 = 95%, 1.21 = 121%）',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_product_code_datetime` (`product_code`, `datetime`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='価格履歴テーブル';

