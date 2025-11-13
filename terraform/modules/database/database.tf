resource "aws_db_instance" "crypto_trading_db" {
  identifier                 = "crypto-trading-db"
  engine                     = "mysql"
  engine_version             = "8.4.7"
  instance_class             = "db.t3.micro"
  allocated_storage          = 20
  storage_type               = "gp3"
  username                   = "crypto_trading_root"
  password                   = var.db_password
  db_name                    = var.db_name != "" ? var.db_name : null
  multi_az                   = false
  publicly_accessible        = true
  backup_window              = "03:10-03:40"
  backup_retention_period    = 30
  auto_minor_version_upgrade = false
  deletion_protection        = true
  skip_final_snapshot        = true
  port                       = var.db_port
  apply_immediately          = false
  vpc_security_group_ids     = [aws_security_group.trading_db_sg.id]
  parameter_group_name       = aws_db_parameter_group.crypto_trading_db.name
  option_group_name          = aws_db_option_group.crypto_trading_db.name
  db_subnet_group_name       = aws_db_subnet_group.crypto_trading_db.name

  lifecycle {
    ignore_changes = [password]
  }
}

# Outputs

output "trading_db_sg_id" {
  description = "ID of the trading database security group"
  value       = aws_security_group.trading_db_sg.id
}
