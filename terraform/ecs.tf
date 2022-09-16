resource "aws_ecs_cluster" "ecs_cluster_instagame" {
  name = "insta-game-cluster"
}

resource "aws_ecs_task_definition" "ecs_task_definition_instagame" {
  family                   = "service"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  container_definitions = jsonencode([
    {
      name   = "xonotic"
      image  = "ghcr.io/fogo-sh/insta-game:release-xonotic"
      cpu    = 256
      memory = 512
    }
  ])
}

resource "aws_ecs_service" "ecs_service_instagame" {
  name            = "instagame-service"
  cluster         = aws_ecs_cluster.ecs_cluster_instagame.id
  task_definition = aws_ecs_task_definition.ecs_task_definition_instagame.arn
  desired_count   = 0
  launch_type     = "FARGATE"

  network_configuration {
    assign_public_ip = true
    subnets          = aws_subnet.subnet_instagame[*].id
    security_groups  = [aws_security_group.security_group_instagame.id]
  }
}
