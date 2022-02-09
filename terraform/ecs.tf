#resource "aws_cloudwatch_log_group" "logs" {
#  name              = "insta-game-cluster-logs"
#  retention_in_days = 1
#}

resource "aws_ecs_cluster" "cluster" {
  name = "insta-game-cluster"

  #  configuration {
  #    execute_command_configuration {
  #      logging = "OVERRIDE"
  #
  #      log_configuration {
  #        cloud_watch_log_group_name = aws_cloudwatch_log_group.logs.name
  #      }
  #    }
  #  }
}

resource "aws_ecs_task_definition" "testing" {
  family                   = "service"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  container_definitions = jsonencode([
    {
      name   = "testing"
      image  = "ghcr.io/fogo-sh/insta-game:release-xonotic"
      cpu    = 256
      memory = 512
    }
  ])
}

resource "aws_ecs_service" "testing" {
  name            = "testing-service"
  cluster         = aws_ecs_cluster.cluster.id
  task_definition = aws_ecs_task_definition.testing.arn
  desired_count   = 0
  launch_type     = "FARGATE"

  network_configuration {
    assign_public_ip = true
    subnets          = aws_subnet.insta_game_subnet[*].id
    security_groups  = [aws_security_group.nginx_sg.id]
  }
}
