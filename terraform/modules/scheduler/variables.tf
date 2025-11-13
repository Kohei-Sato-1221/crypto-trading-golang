variable "app_name" {}
variable "environment" {}
variable "start_schedule_expression" {}
variable "stop_schedule_expression" {}
variable "slack_url" {}
variable "function_name" {
  default = "aws-scheduler"
}
