variable "public_key_path" {}

resource "aws_key_pair" "key_pair" {
  key_name = "tf-20210724"
  public_key = "${file(var.public_key_path)}"
}