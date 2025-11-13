-- Create buy_orders table
CREATE TABLE `buy_orders` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `order_id` varchar(50) DEFAULT NULL,
  `product_code` varchar(50) DEFAULT NULL,
  `side` varchar(20) DEFAULT NULL,
  `price` float DEFAULT NULL,
  `size` float DEFAULT NULL,
  `exchange` varchar(50) DEFAULT NULL,
  `filled` tinyint(4) DEFAULT '0' COMMENT '0:unfilled / 1:filled',
  `status` varchar(20) DEFAULT 'UNFILLED' COMMENT 'UNFILLED / FILLED / CANCELLED',
  `strategy` tinyint(4) DEFAULT '99' COMMENT '99:not recorded ',
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updatetime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `orderId` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='bitflyer_buyorders';

-- Create sell_orders table
CREATE TABLE `sell_orders` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `parentid` varchar(50) DEFAULT NULL,
  `order_id` varchar(50) DEFAULT NULL,
  `product_code` varchar(50) DEFAULT NULL,
  `side` varchar(20) DEFAULT NULL,
  `price` float DEFAULT NULL,
  `size` float DEFAULT NULL,
  `exchange` varchar(50) DEFAULT NULL,
  `filled` tinyint(4) DEFAULT '0' COMMENT '0:unfilled / 1:filled',
  `status` varchar(20) DEFAULT 'UNFILLED' COMMENT 'UNFILLED / FILLED / CANCELLED',
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updatetime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `orderId` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='bitflyer_sellorders';

