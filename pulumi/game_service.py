import json

import pulumi
import pulumi_aws as aws


class GameService(pulumi.ComponentResource):
    """
    Reusable component for a game server hosted on ECS Fargate with a sidecar.
    Equivalent to terraform/modules/game_service.
    """

    service_name: pulumi.Output[str]

    def __init__(
        self,
        name: str,
        game_name: str,
        name_prefix: str,
        image: str,
        cluster_id: pulumi.Input[str],
        cluster_name: pulumi.Input[str],
        subnet_ids: list,
        security_group_id: pulumi.Input[str],
        task_role_arn: pulumi.Input[str],
        execution_role_arn: pulumi.Input[str],
        sidecar_token: pulumi.Input[str],
        data_url: str | None = None,
        game_port: int = 26000,
        game_port_protocols: list[str] | None = None,
        extra_port_mappings: list[dict[str, int | str]] | None = None,
        sidecar_port: int = 5001,
        cpu: int = 256,
        memory: int = 512,
        idle_timeout_seconds: int = 600,
        protocol: str = "xonotic",
        game_cmd: str = "",
        game_args: pulumi.Input[str] = "",
        game_quit_cmd: str = "quit",
        game_quit_timeout: int = 15,
        config_path: str = "/opt/data/server.cfg",
        rcon_password: pulumi.Input[str] | None = None,
        cpu_architecture: str = "X86_64",
        opts: pulumi.ResourceOptions = None,
    ):
        super().__init__("insta-game:index:GameService", name, {}, opts)

        child_opts = pulumi.ResourceOptions(parent=self)
        region = aws.get_region().region
        service_name = f"{name_prefix}-{game_name}-service"
        task_family = f"{name_prefix}-{game_name}"
        if game_port_protocols is None:
            game_port_protocols = ["udp"]
        if extra_port_mappings is None:
            extra_port_mappings = []

        log_group = aws.cloudwatch.LogGroup(
            f"{name}-log-group",
            name=f"/ecs/{name_prefix}/{game_name}",
            retention_in_days=7,
            opts=child_opts,
        )

        container_defs = pulumi.Output.all(
            log_group.name,
            sidecar_token,
            cluster_name,
            game_args,
            rcon_password,
        ).apply(
            lambda args: json.dumps(
                [
                    {
                        "name": game_name,
                        "image": image,
                        "cpu": cpu,
                        "memory": memory,
                        "portMappings": [
                            *[
                                {"containerPort": game_port, "protocol": protocol_name}
                                for protocol_name in game_port_protocols
                            ],
                            *extra_port_mappings,
                            {"containerPort": sidecar_port, "protocol": "tcp"},
                        ],
                        "environment": [
                            {"name": "AWS_REGION", "value": region},
                            {"name": "ECS_CLUSTER", "value": args[2]},
                            {"name": "ECS_SERVICE", "value": service_name},
                            {"name": "TOKEN", "value": args[1]},
                            {"name": "IDLE_TIMEOUT_SECONDS", "value": str(idle_timeout_seconds)},
                            {"name": "GAME_PORT", "value": str(game_port)},
                            {"name": "PROTOCOL", "value": protocol},
                            {"name": "GAME_CMD", "value": game_cmd},
                            {"name": "GAME_ARGS", "value": args[3]},
                            {"name": "GAME_QUIT_CMD", "value": game_quit_cmd},
                            {"name": "GAME_QUIT_TIMEOUT", "value": str(game_quit_timeout)},
                            {"name": "CONFIG_PATH", "value": config_path},
                            *([{"name": "RCON_PASSWORD", "value": args[4]}] if args[4] else []),
                            *([{"name": "DATA_URL", "value": data_url}] if data_url else []),
                        ],
                        "logConfiguration": {
                            "logDriver": "awslogs",
                            "options": {
                                "awslogs-group": args[0],
                                "awslogs-region": region,
                                "awslogs-stream-prefix": "ecs",
                            },
                        },
                    }
                ]
            )
        )

        task_def = aws.ecs.TaskDefinition(
            f"{name}-task-def",
            family=task_family,
            requires_compatibilities=["FARGATE"],
            network_mode="awsvpc",
            cpu=str(cpu),
            memory=str(memory),
            task_role_arn=task_role_arn,
            execution_role_arn=execution_role_arn,
            container_definitions=container_defs,
            runtime_platform=aws.ecs.TaskDefinitionRuntimePlatformArgs(
                cpu_architecture=cpu_architecture,
                operating_system_family="LINUX",
            ),
            opts=child_opts,
        )

        service = aws.ecs.Service(
            f"{name}-service",
            name=service_name,
            cluster=cluster_id,
            task_definition=task_def.arn,
            desired_count=0,
            launch_type="FARGATE",
            network_configuration=aws.ecs.ServiceNetworkConfigurationArgs(
                assign_public_ip=True,
                subnets=subnet_ids,
                security_groups=[security_group_id],
            ),
            opts=child_opts,
        )

        self.service_name = service.name
        self.register_outputs({"service_name": self.service_name})
