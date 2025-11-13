-- Update status column comment and size in buy_orders table
ALTER TABLE `buy_orders` MODIFY COLUMN `status` varchar(20) DEFAULT 'UNFILLED' COMMENT 'UNFILLED / FILLED / FILLED(SELL ORDER PLACED) / CANCELLED';

-- Update status column size in sell_orders table
ALTER TABLE `sell_orders` MODIFY COLUMN `status` varchar(20) DEFAULT 'UNFILLED' COMMENT 'UNFILLED / FILLED / CANCELLED';

