import ipaddress
import json

import pulumi
import pulumi_aws as aws

from game_service import GameService

config = pulumi.Config()
sidecar_token = config.require_secret("sidecarToken")
cidr_block = config.get("cidrBlock") or "172.16.0.0/16"
default_data_url = config.get("defaultDataUrl")
xonotic_data_url = config.get("xonoticDataUrl") or default_data_url
qss_m_data_url = config.get("qssmDataUrl") or default_data_url
q2repro_data_url = config.get("q2reproDataUrl") or default_data_url
region_code = aws.get_region().region
account_id = aws.get_caller_identity().account_id
account_suffix = account_id[-6:]


def regional_name(*parts: str) -> str:
    return "-".join(["insta-game", *parts, region_code])


def global_name(*parts: str) -> str:
    return "-".join(["insta-game", *parts, region_code, account_suffix])


# ---- VPC ----

azs = ["ca-central-1a", "ca-central-1b", "ca-central-1d"]
base_network = ipaddress.IPv4Network(cidr_block)
subnet_cidrs = [str(s) for s in list(base_network.subnets(prefixlen_diff=8))[: len(azs)]]

vpc = aws.ec2.Vpc(
    "game-vpc",
    cidr_block=cidr_block,
    tags={"Name": regional_name("vpc")},
)

subnets = [
    aws.ec2.Subnet(
        f"game-subnet-{i}",
        vpc_id=vpc.id,
        availability_zone=az,
        cidr_block=subnet_cidrs[i],
        tags={"Name": regional_name(f"subnet-{az}")},
    )
    for i, az in enumerate(azs)
]

igw = aws.ec2.InternetGateway(
    "game-igw",
    vpc_id=vpc.id,
    tags={"Name": regional_name("igw")},
)

route_table = aws.ec2.RouteTable(
    "game-route-table",
    vpc_id=vpc.id,
    routes=[
        aws.ec2.RouteTableRouteArgs(cidr_block="0.0.0.0/0", gateway_id=igw.id),
        aws.ec2.RouteTableRouteArgs(ipv6_cidr_block="::/0", gateway_id=igw.id),
    ],
    tags={"Name": regional_name("public-rt")},
)

for i, subnet in enumerate(subnets):
    aws.ec2.RouteTableAssociation(
        f"game-rta-{i}",
        route_table_id=route_table.id,
        subnet_id=subnet.id,
    )

security_group = aws.ec2.SecurityGroup(
    "game-security-group",
    vpc_id=vpc.id,
    name=regional_name("game-sg"),
    ingress=[
        aws.ec2.SecurityGroupIngressArgs(
            from_port=5001,
            to_port=5001,
            protocol="tcp",
            cidr_blocks=["0.0.0.0/0"],
        ),
        aws.ec2.SecurityGroupIngressArgs(
            from_port=26000,
            to_port=26000,
            protocol="udp",
            cidr_blocks=["0.0.0.0/0"],
        ),
    ],
    egress=[
        aws.ec2.SecurityGroupEgressArgs(
            from_port=0,
            to_port=0,
            protocol="-1",
            cidr_blocks=["0.0.0.0/0"],
            ipv6_cidr_blocks=["::/0"],
        ),
    ],
    tags={"Name": regional_name("game-sg")},
)

# ---- IAM ----

# Lambda role — controls ECS service lifecycle
lambda_role = aws.iam.Role(
    "lambda-role",
    name=global_name("lambda-role"),
    assume_role_policy=json.dumps(
        {
            "Version": "2012-10-17",
            "Statement": [
                {
                    "Action": "sts:AssumeRole",
                    "Principal": {"Service": "lambda.amazonaws.com"},
                    "Effect": "Allow",
                }
            ],
        }
    ),
)

lambda_policy = aws.iam.Policy(
    "lambda-policy",
    name=global_name("lambda-policy"),
    policy=json.dumps(
        {
            "Version": "2012-10-17",
            "Statement": [
                {
                    "Effect": "Allow",
                    "Action": [
                        "ecs:ListServices",
                        "ecs:ListTasks",
                        "ecs:DescribeTasks",
                        "ecs:UpdateService",
                        "ec2:DescribeNetworkInterfaces",
                    ],
                    "Resource": "*",
                }
            ],
        }
    ),
)

aws.iam.RolePolicyAttachment(
    "lambda-role-policy",
    role=lambda_role.name,
    policy_arn=lambda_policy.arn,
)

# ECS task execution role — allows Fargate to pull images and write logs
ecs_execution_role = aws.iam.Role(
    "ecs-execution-role",
    name=global_name("ecs-execution-role"),
    assume_role_policy=json.dumps(
        {
            "Version": "2012-10-17",
            "Statement": [
                {
                    "Action": "sts:AssumeRole",
                    "Principal": {"Service": "ecs-tasks.amazonaws.com"},
                    "Effect": "Allow",
                }
            ],
        }
    ),
)

aws.iam.RolePolicyAttachment(
    "ecs-execution-role-policy",
    role=ecs_execution_role.name,
    policy_arn="arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy",
)

# ECS task role — permissions for application code inside the container
ecs_task_role = aws.iam.Role(
    "ecs-task-role",
    name=global_name("ecs-task-role"),
    assume_role_policy=json.dumps(
        {
            "Version": "2012-10-17",
            "Statement": [
                {
                    "Action": "sts:AssumeRole",
                    "Principal": {"Service": "ecs-tasks.amazonaws.com"},
                    "Effect": "Allow",
                }
            ],
        }
    ),
)

ecs_task_policy = aws.iam.Policy(
    "ecs-task-policy",
    name=global_name("ecs-task-policy"),
    policy=json.dumps(
        {
            "Version": "2012-10-17",
            "Statement": [
                {
                    "Effect": "Allow",
                    "Action": ["ecs:UpdateService"],
                    "Resource": "*",
                },
            ],
        }
    ),
)

aws.iam.RolePolicyAttachment(
    "ecs-task-role-policy",
    role=ecs_task_role.name,
    policy_arn=ecs_task_policy.arn,
)

# ---- ECS ----

cluster = aws.ecs.Cluster("game-cluster", name=regional_name("cluster"))

xonotic = GameService(
    "xonotic-arm",
    game_name="xonotic-arm",
    name_prefix=regional_name("game"),
    image="ghcr.io/fogo-sh/insta-game:xonotic",
    cluster_id=cluster.id,
    cluster_name=cluster.name,
    subnet_ids=[s.id for s in subnets],
    security_group_id=security_group.id,
    task_role_arn=ecs_task_role.arn,
    execution_role_arn=ecs_execution_role.arn,
    sidecar_token=sidecar_token,
    cpu=512,
    memory=1024,
    cpu_architecture="ARM64",
    data_url=xonotic_data_url,
    protocol="xonotic",
    game_cmd="./xonotic-linux-arm64-dedicated",
    game_args="",
    game_quit_cmd="exit",
    game_quit_timeout=30,
)

qssm = GameService(
    "qssm-arm",
    game_name="qssm-arm",
    name_prefix=regional_name("game"),
    image="ghcr.io/fogo-sh/insta-game:qssm",
    cluster_id=cluster.id,
    cluster_name=cluster.name,
    subnet_ids=[s.id for s in subnets],
    security_group_id=security_group.id,
    task_role_arn=ecs_task_role.arn,
    execution_role_arn=ecs_execution_role.arn,
    sidecar_token=sidecar_token,
    cpu=512,
    memory=1024,
    cpu_architecture="ARM64",
    data_url=qss_m_data_url,
    protocol="quake1",
    game_cmd="./qssm",
    game_args="-dedicated 12 -basedir /opt -game id1 -port 26000 +exec server.cfg",
    game_quit_cmd="quit",
    game_quit_timeout=15,
    config_path="/opt/id1/server.cfg",
)

q2repro = GameService(
    "q2repro",
    game_name="q2repro",
    name_prefix=regional_name("game"),
    image="ghcr.io/fogo-sh/insta-game:q2repro",
    cluster_id=cluster.id,
    cluster_name=cluster.name,
    subnet_ids=[s.id for s in subnets],
    security_group_id=security_group.id,
    task_role_arn=ecs_task_role.arn,
    execution_role_arn=ecs_execution_role.arn,
    sidecar_token=sidecar_token,
    cpu=512,
    memory=1024,
    cpu_architecture="ARM64",
    data_url=q2repro_data_url,
    protocol="quake2",
    game_cmd="./q2proded",
    game_args="+set dedicated 1 +set basedir /opt +set game baseq2 +set net_ip 0.0.0.0 +set net_port 26000 +set maxclients 12 +exec server.cfg",
    game_quit_cmd="quit",
    game_quit_timeout=15,
    config_path="/opt/baseq2/server.cfg",
)

# ---- Lambda ----

launcher = aws.lambda_.Function(
    "launcher",
    name=regional_name("launcher"),
    runtime="python3.12",
    handler="launcher.handler",
    timeout=120,
    role=lambda_role.arn,
    code=pulumi.AssetArchive(
        {
            "launcher.py": pulumi.FileAsset("../lambda/launcher/launcher.py"),
        }
    ),
    environment=aws.lambda_.FunctionEnvironmentArgs(
        variables=pulumi.Output.all(
            sidecar_token,
            xonotic.service_name,
            cluster.name,
            qssm.service_name,
            q2repro.service_name,
        ).apply(
            lambda args: {
                "SIDECAR_TOKEN": args[0],
                "ECS_CLUSTER": args[2],
                "GAMES": json.dumps(
                    {
                        "xonotic": {
                            "service_name": args[1],
                            "sidecar_port": 5001,
                        },
                        "qssm": {
                            "service_name": args[3],
                            "sidecar_port": 5001,
                        },
                        "q2repro": {
                            "service_name": args[4],
                            "sidecar_port": 5001,
                        },
                    }
                ),
            }
        ),
    ),
)

launcher_url = aws.lambda_.FunctionUrl(
    "launcher-url",
    function_name=launcher.name,
    authorization_type="NONE",
)

aws.lambda_.Permission(
    "launcher-url-permission",
    action="lambda:InvokeFunctionUrl",
    function=launcher.name,
    principal="*",
    function_url_auth_type="NONE",
)

aws.lambda_.Permission(
    "launcher-function-url-invoke-permission",
    action="lambda:InvokeFunction",
    function=launcher.name,
    principal="*",
    invoked_via_function_url=True,
)

pulumi.export("prod_url", launcher_url.function_url)
