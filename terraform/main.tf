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

		touch /home/ec2-user/.ssh/id_rsa
		echo "-----BEGIN OPENSSH PRIVATE KEY-----" >> /home/ec2-user/.ssh/id_rsa
		# NOTE: You have to complement rest of private key before using terraform.
		echo "-----END OPENSSH PRIVATE KEY-----" >> /home/ec2-user/.ssh/id_rsa
		chmod 600 /home/ec2-user/.ssh/id_rsa
		
		mkdir -p /home/ec2-user/tradingapp
		mkdir -p /home/ec2-user/go/src/github.com/Kohei-Sato-1221/crypto-trading-golang
		git clone https://github.com/Kohei-Sato-1221/crypto-trading-golang.git /home/ec2-user/go/src/github.com/Kohei-Sato-1221/crypto-trading-golang
		
		cd /home/ec2-user/go/src/github.com/Kohei-Sato-1221/crypto-trading-golang
		touch build.sh
		chmod 755 build.sh
		echo '#!/bin/bash' > build.sh
		echo '' >> build.sh
		echo 'export GOPATH=/home/ec2-user/go' >> build.sh
		echo 'go get' >> build.sh
		echo 'go build -o main main.go' >> build.sh
		echo 'cp main /home/ec2-user/tradingapp/main' >> build.sh
		echo 'cp [sample]private_config.ini /home/ec2-user/tradingapp/private_config.ini' >> build.sh
		echo 'cp config.ini /home/ec2-user/tradingapp/config.ini' >> build.sh
		# TODO: execute build shell in user data
		# sh ./build.sh

		cd /home/ec2-user/tradingapp
		echo '#!/bin/bash' >> start.sh
		echo "" >> start.sh
		echo "nohup ./main &" >> start.sh
		echo "nohup ./main &" >> start.sh

		echo '#!/bin/bash' >> backup.sh
		echo "" >> backup.sh
		echo "cp /home/ec2-user/tradingapp/trading.log /home/ec2-user/tradingapp/trading_bk.log" >> backup.sh
		echo 'echo "start logging!" > /home/ec2-user/tradingapp/trading.log' >> backup.sh

		echo '#!/bin/bash' >> processCheck.sh
		echo '' >> processCheck.sh
		echo 'count=`ps -ef|grep ./main|wc -l`' >> processCheck.sh
		echo "" >> processCheck.sh
		echo 'echo "go process number:$count"' >> processCheck.sh
		echo "" >> processCheck.sh
		echo 'if [ $count -lt 2 ]; then' >> processCheck.sh
		echo '    echo "Go Application down"' >> processCheck.sh
		echo '    ./main > trading.log &' >> processCheck.sh
		echo 'else' >> processCheck.sh
		echo '    echo "Go Application running!!"' >> processCheck.sh
		echo 'fi' >> processCheck.sh

		yum remove -y mariadb-libs
		yum localinstall -y https://dev.mysql.com/get/mysql80-community-release-el7-3.noarch.rpm
		yum install -y --enablerepo=mysql80-community mysql-community-server
		yum install -y --enablerepo=mysql80-community mysql-community-devel
		touch /var/log/mysqld.log
		systemctl start mysqld
		systemctl enable mysqld

		echo '# ZONE="UTC"' > /etc/sysconfig/clock
		echo 'ZONE="Japan"' >> /etc/sysconfig/clock
		echo 'UTC=true' >> /etc/sysconfig/clock
		ln -sf /usr/share/zoneinfo/Japan /etc/localtime
		## you have to reboot!
EOF
}