locals {
  azs = ["ca-central-1a", "ca-central-1b", "ca-central-1d"]
}

resource "aws_vpc" "vpc_instagame" {
  cidr_block = var.cidr_block
}

resource "aws_subnet" "subnet_instagame" {
  count = length(local.azs)

  vpc_id            = aws_vpc.vpc_instagame.id
  availability_zone = local.azs[count.index]
  cidr_block        = cidrsubnet(var.cidr_block, 8, count.index)
}

resource "aws_internet_gateway" "internet_gateway_instagame" {
  vpc_id = aws_vpc.vpc_instagame.id
}

resource "aws_route_table" "route_table_instagame" {
  vpc_id = aws_vpc.vpc_instagame.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.internet_gateway_instagame.id
  }

  route {
    ipv6_cidr_block = "::/0"
    gateway_id      = aws_internet_gateway.internet_gateway_instagame.id
  }
}

resource "aws_route_table_association" "route_table_association_instagame" {
  count = length(aws_subnet.subnet_instagame)

  route_table_id = aws_route_table.route_table_instagame.id
  subnet_id      = aws_subnet.subnet_instagame[count.index].id
}

resource "aws_security_group" "security_group_instagame" {
  vpc_id = aws_vpc.vpc_instagame.id

  ingress {
    from_port   = 5001
    to_port     = 5001
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 26000
    to_port     = 26000
    protocol    = "udp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    protocol         = "-1"
    from_port        = 0
    to_port          = 0
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }
}
