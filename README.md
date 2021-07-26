# Crypto Trading Golang
Automated Crypto currency trading web application implemented by GoLang

This application place buy orders at the specifig times in a day, checks if they're filled.
If they're, it places sell orders at a liite higher price of buy orders.(currencty +1.5% is hard coded)

## Supported Currencies & Exchange
1. bitflyer(BTC, ETH)
2. OKEX(BTC,ETH,BCH,EOS,BSV,OKB)

※ Spot Trading only. Margin or FX trading are not supported.

  
## How to Build
1. simple build
```go build main.go```

2. build for Amazon Linux
```GOOS=linux GOARCH=amd64 go build main.go```

  
## How to use it
1. In order to select exchange, modify src/main/main.go
   Currenty, you can choose bitflyer(jp) or OKEX for trading.

2. Prepare MySQL Server. And execute following create sentences.
```
//for bitflyer
CREATE TABLE `buy_orders` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `order_id` varchar(50) DEFAULT NULL,
  `product_code` varchar(50) DEFAULT NULL,
  `side` varchar(20) DEFAULT NULL,
  `price` float DEFAULT NULL,
  `size` float DEFAULT NULL,
  `exchange` varchar(50) DEFAULT NULL,
  `filled` tinyint(4) DEFAULT '0' COMMENT '0:unfilled / 1:filled',
  `strategy` tinyint(4) DEFAULT '99' COMMENT '99:not recorded ',
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updatetime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `orderId` (`order_id`)
) ENGINE=InnoDB AUTO_INCREMENT=7305 DEFAULT CHARSET=latin1 COMMENT='bitflyer_buyorders';

CREATE TABLE `sell_orders` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `parentid` varchar(50) DEFAULT NULL,
  `order_id` varchar(50) DEFAULT NULL,
  `product_code` varchar(50) DEFAULT NULL,
  `side` varchar(20) DEFAULT NULL,
  `price` float DEFAULT NULL,
  `size` float DEFAULT NULL,
  `exchange` varchar(50) DEFAULT NULL,
  `filled` tinyint(4) DEFAULT '0' COMMENT '0:unfilled / 1:filled',
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updatetime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `orderId` (`order_id`)
) ENGINE=InnoDB AUTO_INCREMENT=4261 DEFAULT CHARSET=latin1 COMMENT='bitflyer_sellorders';


// for OKEX
CREATE TABLE `buy_orders` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `order_id` varchar(50) DEFAULT NULL,
  `pair` varchar(50) DEFAULT NULL,
  `side` varchar(25) DEFAULT NULL,
  `price` float DEFAULT NULL,
  `size` float DEFAULT NULL,
  `exchange` varchar(50) DEFAULT NULL,
  `state` tinyint(4) DEFAULT '0' COMMENT '0:unfilled / 1:filled',
  `sell_order_id` varchar(50) DEFAULT NULL,
  `sell_order_state` tinyint(4) DEFAULT '0',
  `sell_price` float DEFAULT NULL,
  `timestamp` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updatetime` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `oderid` (`order_id`)
) ENGINE=InnoDB AUTO_INCREMENT=6569 DEFAULT CHARSET=latin1 COMMENT='okex_orders(buy&sell)';
```

※ You cannot trade with both bitflyer and OKEX with a same mysql server because these two trading logics need talbe name "buy orders". (In the near future, I'll fix source code in order that both two logics can be used with a signle mysql server)

3. Prepare private_config.ini file and locate it to the same directory as go executable file. 
   To do this, you can refer to [sample]private_config.ini in the github repository.(remove [sample] from file name. And input parameters in thie file.)
   
4. Change paramters in config.ini in accordance with your setting.

5. Build this project. Pleaes refer to above [How to build section]

6. execute main file.


# Terraform
```
// deploy to AWS
cd terraform
terraform plan -var 'public_key_path=~/.ssh/tf-20210724.pub'
terraform apply -var 'public_key_path=~/.ssh/tf-20210724.pub'

// destory resources on AWS
terraform destory -var 'public_key_path=~/.ssh/tf-20210724.pub'

// set cron
1. after login EC2 vis ssh, execute following command:
(crontab -l 2>/dev/null; echo "10 17 * * * /home/ec2-user/tradingapp/backup.sh") | crontab -
(crontab -l 2>/dev/null; echo "10 * * * * echo "" > /home/ec2-user/tradingapp/nohup.out") | crontab -
(crontab -l 2>/dev/null; echo "25 * * * * cd /home/ec2-user/tradingapp && sh processCheck.sh > cron.log") | crontab -

// build application
cd /home/ec2-user/go/src/github.com/Kohei-Sato-1221/crypto-trading-golang
sh ./build.sh

// mysql
1. After deployed, go ec2 instance via SSH
2. Execute this command,
`sudo more /var/log/mysqld.log|grep password`
3. You can get first root password of mysql
4. login: `mysql -u root -p` with password you got
5. execute following commands in mysql
  `ALTER USER 'root'@'localhost' IDENTIFIED BY'newPassword';`
  `GRANT ALL ON *.* TO root@localhost;`
  `CREATE USER trading IDENTIFIED BY 'password';`
  `CREATE DATABASE trading;`
  `GRANT ALL PRIVILEGES ON trading.* to 'trading'@'%';`

// run application
sh ./home/ec2-user/tradingapp/start.sh

// memo
show user:
`select user, host, plugin from mysql.user;`

access from Sequel pro:
`ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY 'pasword';`
```