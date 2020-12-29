# BitcoinTrading_Golang
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
  `orderId` varchar(50) DEFAULT NULL,
  `product_code` varchar(50) DEFAULT NULL,
  `side` varchar(20) DEFAULT NULL,
  `price` float DEFAULT NULL,
  `size` float DEFAULT NULL,
  `exchange` varchar(50) DEFAULT NULL,
  `filled` tinyint(4) DEFAULT '0' COMMENT '0:unfilled / 1:filled',
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updatetime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `orderId` (`orderId`)
) ENGINE=InnoDB AUTO_INCREMENT=0 DEFAULT CHARSET=latin1 COMMENT='bitflyer_buyorders';

CREATE TABLE `sell_orders` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `parentid` varchar(50) DEFAULT NULL,
  `orderId` varchar(50) DEFAULT NULL,
  `product_code` varchar(50) DEFAULT NULL,
  `side` varchar(20) DEFAULT NULL,
  `price` float DEFAULT NULL,
  `size` float DEFAULT NULL,
  `exchange` varchar(50) DEFAULT NULL,
  `filled` tinyint(4) DEFAULT '0' COMMENT '0:unfilled / 1:filled',
  `timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updatetime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `orderId` (`orderId`)
) ENGINE=InnoDB AUTO_INCREMENT=0 DEFAULT CHARSET=latin1 COMMENT='bitflyer_sellorders';


// for OKEX
CREATE TABLE `buy_orders` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `orderId` varchar(50) DEFAULT NULL,
  `pair` varchar(50) DEFAULT NULL,
  `side` varchar(25) DEFAULT NULL,
  `price` float DEFAULT NULL,
  `size` float DEFAULT NULL,
  `exchange` varchar(50) DEFAULT NULL,
  `state` tinyint(4) DEFAULT '0' COMMENT '0:unfilled / 1:filled',
  `sellOrderId` varchar(50) DEFAULT NULL,
  `sellOrderState` tinyint(4) DEFAULT '0',
  `sellPrice` float DEFAULT NULL,
  `timestamp` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updatetime` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `oderid` (`orderId`)
) ENGINE=InnoDB AUTO_INCREMENT=0 DEFAULT CHARSET=latin1 COMMENT='okex_orders(buy&sell)';
```

※ You cannot trade with both bitflyer and OKEX with a same mysql server because these two trading logics need talbe name "buy orders". (In the near future, I'll fix source code in order that both two logics can be used with a signle mysql server)

3. Prepare private_config.ini file and locate it to the same directory as go executable file. 
   To do this, you can refer to [sample]private_config.ini in the github repository.(remove [sample] from file name. And input parameters in thie file.)
   
4. Change paramters in config.ini in accordance with your setting.

5. Build this project. Pleaes refer to above [How to build section]

6. execute main file.


