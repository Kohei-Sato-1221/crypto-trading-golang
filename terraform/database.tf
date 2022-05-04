variable "db_password" {}

resource "aws_db_parameter_group" "crypto_trading_db" {
  name   = "crypto-trading-db"
  family = "mysql5.7"

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
  major_engine_version = "5.7"

  option {
    option_name = "MARIADB_AUDIT_PLUGIN"
  }
}

resource "aws_db_subnet_group" "crypto_trading_db" {
  name       = "crypto-trading-db"
  subnet_ids = ["subnet-07c2ad4b0c24e29f0", "subnet-05514fe8997e7b3b1"]
}

resource "aws_db_instance" "crypto_trading_db" {
  identifier                 = "crypto-trading-db"
  engine                     = "mysql"
  engine_version             = "5.7.23"
  instance_class             = "db.t2.micro"
  allocated_storage          = 20
  storage_type               = "gp2"
  username                   = "root"
  password                   = var.db_password
  multi_az                   = false
  publicly_accessible        = true
  backup_window              = "03:10-03:40"
  backup_retention_period    = 30
  auto_minor_version_upgrade = false
  deletion_protection        = true
  skip_final_snapshot        = true
  port                       = 3306
  apply_immediately          = false
  vpc_security_group_ids     = [aws_security_group.trading_db_sg.id]
  parameter_group_name       = aws_db_parameter_group.crypto_trading_db.name
  option_group_name          = aws_db_option_group.crypto_trading_db.name
  db_subnet_group_name       = aws_db_subnet_group.crypto_trading_db.name

  lifecycle {
    ignore_changes = [password]
  }
}