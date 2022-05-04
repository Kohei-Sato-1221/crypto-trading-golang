variable "public_key_path" {}

resource "aws_key_pair" "key_pair" {
  key_name   = "sugar-2022-keypair"
  public_key = file(var.public_key_path)
}