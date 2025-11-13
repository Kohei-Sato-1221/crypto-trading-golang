-- Remove filled column from buy_orders table
ALTER TABLE `buy_orders` DROP COLUMN `filled`;

-- Remove filled column from sell_orders table
ALTER TABLE `sell_orders` DROP COLUMN `filled`;

