-- Add remarks column to buy_orders table
ALTER TABLE `buy_orders` ADD COLUMN `remarks` TEXT DEFAULT NULL;

-- Add remarks column to sell_orders table
ALTER TABLE `sell_orders` ADD COLUMN `remarks` TEXT DEFAULT NULL;

