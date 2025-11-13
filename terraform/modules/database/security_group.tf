# Trading DB Security Group
resource "aws_security_group" "trading_db_sg" {
  name        = "trading_db_sg"
  description = "Security group for trading database"
  vpc_id      = var.vpc_id

  tags = merge(
    var.tags,
    {
      Name = "trading_db_sg"
    }
  )
}

# Ingress rules for DB (Public access allowed)
resource "aws_security_group_rule" "ingress_db" {
  type              = "ingress"
  from_port         = var.db_port
  to_port           = var.db_port
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.trading_db_sg.id
  description       = "MySQL access from public (0.0.0.0/0)"
}