resource "aws_security_group" "trading_server_ec2_sg" {
    name = "trading_server_ec2_sg"
}

resource "aws_security_group_rule" "ingress_ssh" {
    type = "ingress"
    from_port = 22
    to_port = 22
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    security_group_id = aws_security_group.trading_server_ec2_sg.id
}

resource "aws_security_group_rule" "ingress_mysql" {
    type = "ingress"
    from_port = 3306
    to_port = 3306
    protocol = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    security_group_id = aws_security_group.trading_server_ec2_sg.id
}

resource "aws_security_group_rule" "sugar_egress" {
    type = "egress"
    from_port = "0"
    to_port = "0"
    protocol = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    security_group_id = aws_security_group.trading_server_ec2_sg.id
}
