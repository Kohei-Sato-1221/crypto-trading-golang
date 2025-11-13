terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.17.0"
    }
  }

  backend "s3" {
    region                  = "ap-northeast-1"
    bucket                  = "tfstate-crypto-trading-20251113"
    key                     = "terraform.tfstate"
    profile                 = "crypto-trading-20251113"
    shared_credentials_file = "~/.aws/credentials"
  }

  required_version = "= 1.13.5"
}

provider "aws" {
  profile                  = "crypto-trading-20251113"
  region                   = "ap-northeast-1"
  shared_config_files      = ["~/.aws/config"]
  shared_credentials_files = ["~/.aws/credentials"]
}

# Variables
variable "db_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}

variable "db_name" {
  description = "Database name"
  type        = string
  default     = "crypto_trading_db"
}

variable "db_port" {
  description = "Database port"
  type        = string
  default     = 3306
}

variable "slack_url" {
  description = "Slack Incoming Webhook URL"
  type        = string
  sensitive   = true
}

# ネットワークモジュール（VPC、Subnet、RouteTableを統合）
module "network" {
  source = "./modules/network"

  vpc_cidr             = "210.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  vpc_tags = {
    Environment = "production"
    Project     = "crypto-trading"
  }

  subnets = [
    {
      name                    = "crypto-trading-public-subnet-1a"
      cidr                    = "210.0.1.0/24"
      availability_zone       = "ap-northeast-1a"
      map_public_ip_on_launch = true
      tags = {
        Name        = "crypto-trading-public-subnet-1a"
        Type        = "public"
        Environment = "production"
        Project     = "crypto-trading"
      }
    },
    {
      name                    = "crypto-trading-public-subnet-1c"
      cidr                    = "210.0.2.0/24"
      availability_zone       = "ap-northeast-1c"
      map_public_ip_on_launch = true
      tags = {
        Name        = "crypto-trading-public-subnet-1c"
        Type        = "public"
        Environment = "production"
        Project     = "crypto-trading"
      }
    },
    {
      name                    = "crypto-trading-private-subnet-1a"
      cidr                    = "210.0.3.0/24"
      availability_zone       = "ap-northeast-1a"
      map_public_ip_on_launch = false
      tags = {
        Name        = "crypto-trading-private-subnet-1a"
        Type        = "private"
        Environment = "production"
        Project     = "crypto-trading"
      }
    },
    {
      name                    = "crypto-trading-private-subnet-1c"
      cidr                    = "210.0.4.0/24"
      availability_zone       = "ap-northeast-1c"
      map_public_ip_on_launch = false
      tags = {
        Name        = "crypto-trading-private-subnet-1c"
        Type        = "private"
        Environment = "production"
        Project     = "crypto-trading"
      }
    }
  ]

  route_tables = [
    {
      name = "crypto-trading-public-route-table"
      routes = [
        {
          cidr_block = "0.0.0.0/0"
          gateway_id = module.network.internet_gateway_id
        }
      ]
      subnet_names = [
        "crypto-trading-public-subnet-1a",
        "crypto-trading-public-subnet-1c"
      ]
      tags = {
        Name        = "crypto-trading-public-route-table"
        Type        = "public"
        Environment = "production"
        Project     = "crypto-trading"
      }
    },
    {
      name   = "crypto-trading-private-route-table"
      routes = []
      # インターネットアクセスが必要な場合は、NATゲートウェイへのルートを追加
      # routes = [
      #   {
      #     cidr_block     = "0.0.0.0/0"
      #     nat_gateway_id = module.nat_gateway.nat_gateway_id
      #   }
      # ]
      subnet_names = [
        "crypto-trading-private-subnet-1a",
        "crypto-trading-private-subnet-1c"
      ]
      tags = {
        Name        = "crypto-trading-private-route-table"
        Type        = "private"
        Environment = "production"
        Project     = "crypto-trading"
      }
    }
  ]
}

# Databaseモジュール（Security Groupも含む）
module "database" {
  source = "./modules/database"

  db_password = var.db_password
  db_port     = 1221
  db_name     = var.db_name
  vpc_id      = module.network.vpc_id
  subnet_ids = [
    module.network.subnet_ids["crypto-trading-public-subnet-1a"],
    module.network.subnet_ids["crypto-trading-public-subnet-1c"]
  ]

  tags = {
    Environment            = "production"
    Project                = "crypto-trading"
    CryptoTradingScheduler = "true"
  }
}

# Schedulerモジュール
module "scheduler" {
  source = "./modules/scheduler"

  app_name                  = "crypto-trading"
  environment               = "production"
  start_schedule_expression = "cron(0 9 * * ? *)"  # 毎日9時（UTC）に起動
  stop_schedule_expression  = "cron(0 18 * * ? *)" # 毎日18時（UTC）に停止
  slack_url                 = var.slack_url
}
