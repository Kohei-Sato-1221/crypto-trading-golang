resource "aws_db_parameter_group" "crypto_trading_db" {
  name   = "crypto-trading-db"
  family = "mysql8.4"

  parameter {
    name  = "character_set_database"
    value = "utf8mb4"
  }

  parameter {
    name  = "time_zone"
    value = "Asia/Tokyo"
  }

  parameter {
    name  = "lc_time_names"
    value = "ja_JP"
  }
}

resource "aws_db_option_group" "crypto_trading_db" {
  name                 = "crypto-trading-db"
  engine_name          = "mysql"
  major_engine_version = "8.4"

  option {
    option_name = "MARIADB_AUDIT_PLUGIN"
  }
}


resource "aws_db_subnet_group" "crypto_trading_db" {
  name       = "crypto-trading-db"
  subnet_ids = var.subnet_ids
}