provider "aws" {
    region = "ap-northeast-1"
}

locals {
	ec2_tag = "Crypto_Trading_Server"
	instance_type = "t2.micro"
}

data "aws_ami" "recent_amazon_linux_2" {
	most_recent = true
	owners	    = ["amazon"]

	filter {
		name   = "name"
		values = ["amzn2-ami-hvm-2.0.????????-x86_64-gp2"]
	}

	filter {
		name   = "state"
		values = ["available"]
	}
}

resource "aws_instance" "trading_server_ec2" {
  	ami		= data.aws_ami.recent_amazon_linux_2.image_id
	instance_type   = local.instance_type
    key_name = "${aws_key_pair.key_pair.key_name}"
	vpc_security_group_ids = [aws_security_group.trading_server_ec2_sg.id]
	
	tags = {
		Name = local.ec2_tag
	}

	user_data = <<EOF
		#!/bin/bash
		yum install -y go
		yum remove -y mariadb-libs
		yum localinstall -y https://dev.mysql.com/get/mysql80-community-release-el7-3.noarch.rpm
		yum install -y --enablerepo=mysql80-community mysql-community-server
		yum install -y --enablerepo=mysql80-community mysql-community-devel
		touch /var/log/mysqld.log
		systemctl start mysqld
		systemctl enable mysqld
EOF
}