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

2. Prepare RDS instance(MySQL 8.4.7). And execute following sentences.
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
) ENGINE=InnoDB AUTO_INCREMENT=7305 COMMENT='bitflyer_buyorders';

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
) ENGINE=InnoDB AUTO_INCREMENT=4261 COMMENT='bitflyer_sellorders';


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

This project uses Terraform to manage AWS infrastructure for the crypto trading application.

## Directory Structure

```
terraform/
├── main.tf                 # Main Terraform configuration file
├── terraform.tfvars        # Variable values (contains sensitive data)
├── .terraform.lock.hcl     # Provider version lock file
└── modules/
    ├── network/
    │   └── main.tf         # VPC, Subnets, Route Tables
    └── database/
        ├── variables.tf    # Module variables
        ├── database.tf     # RDS instance configuration
        ├── database_params.tf  # DB parameter and option groups
        └── security_group.tf   # Security groups for DB and EC2
```

## Important Notes

1. **Sensitive Data**: The `terraform.tfvars` file contains sensitive information (database passwords). 
   - **DO NOT commit this file to version control**
   - Add `terraform.tfvars` to `.gitignore`
   - Use environment-specific variable files or AWS Secrets Manager for production

2. **Terraform State**: The state file is stored in S3 backend:
   - Bucket: `tfstate-crypto-trading-20251113`
   - Region: `ap-northeast-1`
   - Ensure the S3 bucket exists before running `terraform init`

3. **AWS Profile**: The configuration uses AWS profile `crypto-trading-20251113`
   - Make sure this profile is configured in `~/.aws/credentials` and `~/.aws/config`

4. **Terraform Version**: This project requires Terraform version `1.13.5` exactly
   - Use `tfenv` to manage Terraform versions

5. **Module Structure**:
   - `network` module: Manages VPC, private subnets, and route tables
   - `database` module: Manages RDS MySQL instance, security groups, and related resources

## Using Makefile

The project includes a Makefile to simplify Terraform operations. All commands should be run from the project root directory.

### Available Commands

```bash
# Show all available commands
make help

# Format Terraform code
make fmt

# Initialize Terraform (formats code and runs terraform init)
make init

# Plan infrastructure changes
make plan

# Apply infrastructure changes
make apply
```

### Command Details

- **`make fmt`**: Formats all Terraform files recursively
- **`make init`**: 
  - Formats code
  - Sets Terraform version to 1.13.5
  - Initializes Terraform in the `terraform/` directory
- **`make plan`**: 
  - Formats code
  - Sets Terraform version
  - Shows planned infrastructure changes
- **`make apply`**: 
  - Formats code
  - Sets Terraform version
  - Applies infrastructure changes

### Workflow Example

```bash
# First time setup
make init

# Review changes
make plan

# Apply changes
make apply
```

## Prerequisites

1. **Terraform**: Install Terraform 1.13.5 (recommended: use `tfenv`)
2. **AWS CLI**: Configure AWS credentials with the profile `crypto-trading-20251113`
3. **S3 Bucket**: Create the S3 bucket for Terraform state:
   ```bash
   aws s3 mb s3://tfstate-crypto-trading-20251113 --region ap-northeast-1 --profile crypto-trading-20251113
   ```
4. **tfvars file**: Create `terraform/terraform.tfvars` with required variables:
   ```hcl
   db_password = "your-database-password"
   ```
