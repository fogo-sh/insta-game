locals {
  azs = ["ca-central-1a", "ca-central-1b", "ca-central-1d"]
}

resource "aws_vpc" "insta_game_vpc" {
  cidr_block = var.cidr_block

  tags = {
    Name = "insta-game-vpc"
  }
}

resource "aws_subnet" "insta_game_subnet" {
  count = length(local.azs)

  vpc_id            = aws_vpc.insta_game_vpc.id
  availability_zone = local.azs[count.index]
  cidr_block        = cidrsubnet(var.cidr_block, 8, count.index)

  tags = {
    Name = "insta-game-${local.azs[count.index]}"
  }
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.insta_game_vpc.id

  tags = {
    Name = "insta-game-igw"
  }
}

resource "aws_route_table" "subnet_rt" {
  vpc_id = aws_vpc.insta_game_vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }

  route {
    ipv6_cidr_block = "::/0"
    gateway_id      = aws_internet_gateway.igw.id
  }

  tags = {
    Name = "insta-game-route-table"
  }
}

resource "aws_route_table_association" "subnet_rt" {
  count = length(aws_subnet.insta_game_subnet)

  route_table_id = aws_route_table.subnet_rt.id
  subnet_id      = aws_subnet.insta_game_subnet[count.index].id
}

resource "aws_security_group" "nginx_sg" {
  name_prefix = "insta-game-nginx-sg"
  vpc_id      = aws_vpc.insta_game_vpc.id

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
