# VPC変数
variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "enable_dns_hostnames" {
  description = "Enable DNS hostnames in the VPC"
  type        = bool
  default     = true
}

variable "enable_dns_support" {
  description = "Enable DNS support in the VPC"
  type        = bool
  default     = true
}

variable "vpc_tags" {
  description = "A map of tags to assign to the VPC"
  type        = map(string)
  default     = {}
}

# Subnet変数
variable "subnets" {
  description = "List of subnets to create"
  type = list(object({
    name                    = string
    cidr                    = string
    availability_zone       = string
    map_public_ip_on_launch = bool
    tags                    = map(string)
  }))
  default = []
}

# Route Table変数
variable "route_tables" {
  description = "List of route tables to create"
  type = list(object({
    name = string
    routes = list(object({
      cidr_block           = string
      gateway_id           = optional(string)
      nat_gateway_id       = optional(string)
      network_interface_id = optional(string)
    }))
    subnet_names = list(string) # 関連付けるサブネット名のリスト
    tags         = map(string)
  }))
  default = []
}

# VPCリソース
resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = var.enable_dns_hostnames
  enable_dns_support   = var.enable_dns_support

  tags = merge(
    var.vpc_tags,
    {
      Name = "crypto-trading-vpc"
    }
  )
}

# Internet Gateway
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = merge(
    var.vpc_tags,
    {
      Name = "crypto-trading-igw"
    }
  )
}

# Subnetリソース
resource "aws_subnet" "main" {
  for_each = {
    for subnet in var.subnets : subnet.name => subnet
  }

  vpc_id                  = aws_vpc.main.id
  cidr_block              = each.value.cidr
  availability_zone       = each.value.availability_zone
  map_public_ip_on_launch = each.value.map_public_ip_on_launch

  tags = merge(
    each.value.tags,
    {
      Name = each.value.name
    }
  )
}

# Route Tableリソース
resource "aws_route_table" "main" {
  for_each = {
    for rt in var.route_tables : rt.name => rt
  }

  vpc_id = aws_vpc.main.id

  dynamic "route" {
    for_each = each.value.routes
    content {
      cidr_block = route.value.cidr_block
      # nullでない場合のみ設定（Terraformではnull値の属性は自動的に無視される）
      gateway_id           = try(route.value.gateway_id, null)
      nat_gateway_id       = try(route.value.nat_gateway_id, null)
      network_interface_id = try(route.value.network_interface_id, null)
    }
  }

  tags = merge(
    each.value.tags,
    {
      Name = each.value.name
    }
  )
}

# Route Table Associationリソース
resource "aws_route_table_association" "main" {
  for_each = {
    for pair in flatten([
      for rt in var.route_tables : [
        for subnet_name in rt.subnet_names : {
          key         = "${rt.name}-${subnet_name}"
          route_table = rt.name
          subnet_name = subnet_name
        }
      ]
    ]) : pair.key => pair
  }

  route_table_id = aws_route_table.main[each.value.route_table].id
  subnet_id      = aws_subnet.main[each.value.subnet_name].id
}

# Outputs
output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "vpc_cidr_block" {
  description = "CIDR block of the VPC"
  value       = aws_vpc.main.cidr_block
}

output "subnet_ids" {
  description = "Map of subnet names to subnet IDs"
  value = {
    for k, v in aws_subnet.main : k => v.id
  }
}

output "subnet_cidr_blocks" {
  description = "Map of subnet names to subnet CIDR blocks"
  value = {
    for k, v in aws_subnet.main : k => v.cidr_block
  }
}

output "route_table_ids" {
  description = "Map of route table names to route table IDs"
  value = {
    for k, v in aws_route_table.main : k => v.id
  }
}

output "internet_gateway_id" {
  description = "ID of the Internet Gateway"
  value       = aws_internet_gateway.main.id
}

