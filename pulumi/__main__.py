import ipaddress
import json
from pathlib import Path

import pulumi
import pulumi_aws as aws
import pulumi_random as random

from game_service import GameService

config = pulumi.Config()
web_ui_passphrase = config.require_secret("webUiPassphrase")
api_token = config.require_secret("apiToken")
discord_public_key = config.require_secret("discordPublicKey")
discord_bot_token = config.require_secret("discordBotToken")
discord_app_id = config.get("discordAppId") or ""
budget_alert_email = config.require("budgetAlertEmail")
monthly_budget_limit_usd = config.get_float("monthlyBudgetLimitUsd") or 50
enable_custom_domain = config.get_bool("enableCustomDomain") or False
custom_domain_hostname = (
    config.require("customDomainHostname")
    if enable_custom_domain
    else config.get("customDomainHostname")
)
cidr_block = config.get("cidrBlock") or "172.16.0.0/16"
default_data_url = config.get_secret("defaultDataUrl")
rcon_password = config.require_secret("rconPassword")
launcher_reserved_concurrency = config.get_int("launcherReservedConcurrency")
region_code = aws.get_region().region
account_id = aws.get_caller_identity().account_id
account_suffix = account_id[-6:]

sidecar_token = random.RandomPassword(
    "sidecar-token",
    length=40,
    special=False,
).result


def regional_name(*parts: str) -> str:
    return "-".join(["insta-game", *parts, region_code])


def global_name(*parts: str) -> str:
    return "-".join(["insta-game", *parts, region_code, account_suffix])


def load_game_definitions() -> list[dict]:
    definitions: list[dict] = []
    docker_root = Path(__file__).resolve().parent.parent / "docker-containers"
    for game_dir in sorted(path for path in docker_root.iterdir() if path.is_dir()):
        metadata_path = game_dir / "game.json"
        dockerfile_path = game_dir / "Dockerfile"
        if not metadata_path.exists() or not dockerfile_path.exists():
            continue
        with metadata_path.open() as f:
            definitions.append(json.load(f))
    return definitions


def env_var_to_pulumi_key(env_var: str) -> str:
    parts = env_var.lower().split("_")
    return parts[0] + "".join(part.title() for part in parts[1:])


def data_url_for_game(metadata: dict):
    data_url_env = metadata.get("dataUrlEnv")
    if not data_url_env:
        return None
    return config.get_secret(env_var_to_pulumi_key(data_url_env)) or default_data_url


def parse_port_mapping(port_key: str) -> tuple[int, str]:
    port, protocol = port_key.split("/", 1)
    return int(port), protocol.lower()


def game_port_settings(metadata: dict) -> tuple[int, list[str], list[dict[str, int | str]]]:
    game_port = int(metadata.get("gamePort", 26000))
    sidecar_port = int(metadata.get("sidecarPort", 5001))
    declared_ports = metadata.get("ports", {})

    game_port_protocols: list[str] = []
    extra_port_mappings: list[dict[str, int | str]] = []
    for port_key in sorted(declared_ports):
        container_port, protocol = parse_port_mapping(port_key)
        if container_port == sidecar_port and protocol == "tcp":
            continue
        if container_port == game_port:
            game_port_protocols.append(protocol)
            continue
        extra_port_mappings.append({"containerPort": container_port, "protocol": protocol})

    if not game_port_protocols:
        game_port_protocols = ["udp"]

    return game_port, sorted(set(game_port_protocols)), extra_port_mappings


game_definitions = load_game_definitions()


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
            from_port=port,
            to_port=port,
            protocol=protocol,
            cidr_blocks=["0.0.0.0/0"],
        )
        for port, protocol in sorted(
            {
                parse_port_mapping(port_key)
                for metadata in game_definitions
                for port_key in metadata.get("ports", {})
            }
        )
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
                        "logs:FilterLogEvents",
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

aws.iam.RolePolicyAttachment(
    "lambda-basic-execution-role-policy",
    role=lambda_role.name,
    policy_arn="arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole",
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

game_services: dict[str, GameService] = {}
for metadata in game_definitions:
    game_id = metadata["id"]
    game_port, game_port_protocols, extra_port_mappings = game_port_settings(metadata)
    game_services[game_id] = GameService(
        game_id,
        game_name=game_id,
        name_prefix=regional_name("game"),
        image=metadata.get("image", f"ghcr.io/fogo-sh/insta-game:{game_id}"),
        cluster_id=cluster.id,
        cluster_name=cluster.name,
        subnet_ids=[s.id for s in subnets],
        security_group_id=security_group.id,
        task_role_arn=ecs_task_role.arn,
        execution_role_arn=ecs_execution_role.arn,
        sidecar_token=sidecar_token,
        log_token=web_ui_passphrase,
        data_url=data_url_for_game(metadata),
        game_port=game_port,
        game_port_protocols=game_port_protocols,
        extra_port_mappings=extra_port_mappings,
        sidecar_port=int(metadata.get("sidecarPort", 5001)),
        cpu=int(metadata.get("cpu", 512)),
        memory=int(metadata.get("memory", 1024)),
        cpu_architecture="ARM64",
        game_cmd=metadata.get("gameCmd", ""),
        game_args=metadata.get("gameArgs", ""),
        game_quit_cmd=metadata.get("gameQuitCmd", "quit"),
        game_quit_timeout=int(metadata.get("gameQuitTimeout", 15)),
        config_path=metadata.get("configPath", "/opt/data/server.cfg"),
        rcon_password=rcon_password,
    )

# ---- Lambda ----

launcher_games = {
    metadata["id"]: {
        "serviceName": game_services[metadata["id"]].service_name,
        "sidecarPort": int(metadata.get("sidecarPort", 5001)),
        "logGroupName": game_services[metadata["id"]].log_group_name,
    }
    for metadata in game_definitions
}

launcher_game_ids = [metadata["id"] for metadata in game_definitions]
launcher_game_configs = {
    game_id: pulumi.Output.all(
        service_name=game_services[game_id].service_name,
        log_group_name=game_services[game_id].log_group_name,
    ).apply(
        lambda args, game_id=game_id: {
            "serviceName": args["service_name"],
            "sidecarPort": launcher_games[game_id]["sidecarPort"],
            "logGroupName": args["log_group_name"],
        }
    )
    for game_id in launcher_game_ids
}

launcher = aws.lambda_.Function(
    "launcher",
    name=regional_name("launcher"),
    runtime="nodejs22.x",
    handler="index.handler",
    timeout=120,
    memory_size=512,
    role=lambda_role.arn,
    code=pulumi.FileArchive("../launcher/dist"),
    environment=aws.lambda_.FunctionEnvironmentArgs(
        variables=pulumi.Output.all(
            sidecar_token=sidecar_token,
            cluster_name=cluster.name,
            web_ui_passphrase=web_ui_passphrase,
            api_token=api_token,
            discord_public_key=discord_public_key,
            discord_bot_token=discord_bot_token,
            discord_app_id=discord_app_id,
            games=pulumi.Output.all(**launcher_game_configs),
        ).apply(
            lambda args: {
                "SIDECAR_TOKEN": args["sidecar_token"],
                "ECS_CLUSTER": args["cluster_name"],
                "WEB_UI_PASSPHRASE": args["web_ui_passphrase"],
                "API_TOKEN": args["api_token"],
                "DISCORD_PUBLIC_KEY": args["discord_public_key"],
                "DISCORD_BOT_TOKEN": args["discord_bot_token"],
                "DISCORD_APP_ID": args["discord_app_id"],
                "GAMES": json.dumps(args["games"]),
            }
        ),
    ),
    reserved_concurrent_executions=launcher_reserved_concurrency,
)

launcher_log_group = aws.cloudwatch.LogGroup(
    "launcher-log-group",
    name=launcher.name.apply(lambda name: f"/aws/lambda/{name}"),
    retention_in_days=3,
)

# ---- Budget guardrail ----

monthly_budget = aws.budgets.Budget(
    "monthly-budget",
    name=global_name("monthly-budget"),
    budget_type="COST",
    limit_amount=f"{monthly_budget_limit_usd:.2f}",
    limit_unit="USD",
    time_unit="MONTHLY",
    notifications=[
        {
            "comparison_operator": "GREATER_THAN",
            "threshold": 50,
            "threshold_type": "PERCENTAGE",
            "notification_type": "ACTUAL",
            "subscriber_email_addresses": [budget_alert_email],
        },
        {
            "comparison_operator": "GREATER_THAN",
            "threshold": 80,
            "threshold_type": "PERCENTAGE",
            "notification_type": "ACTUAL",
            "subscriber_email_addresses": [budget_alert_email],
        },
        {
            "comparison_operator": "GREATER_THAN",
            "threshold": 100,
            "threshold_type": "PERCENTAGE",
            "notification_type": "FORECASTED",
            "subscriber_email_addresses": [budget_alert_email],
        },
    ],
)

launcher_url = aws.lambda_.FunctionUrl(
    "launcher-url",
    function_name=launcher.name,
    authorization_type="NONE",
    invoke_mode="RESPONSE_STREAM",
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

# ---- Custom domain ----
#
# ACM certificate must live in us-east-1 (CloudFront requirement).
# DNS validation: add the CNAME record ACM provides to Cloudflare DNS.
# First deploy with enableCustomDomain=false, add the ACM validation CNAME, wait
# for the certificate to become ISSUED, then set enableCustomDomain=true and
# deploy again. After the cert is issued, add a single CNAME in Cloudflare:
#   <hostname>  CNAME  <cloudfront_domain>  (proxy OFF / grey cloud)

acm_provider = aws.Provider("acm-us-east-1", region="us-east-1")

cert = (
    aws.acm.Certificate(
        "games-cert",
        domain_name=custom_domain_hostname,
        validation_method="DNS",
        opts=pulumi.ResourceOptions(provider=acm_provider),
    )
    if custom_domain_hostname
    else None
)

# AWS managed cache policies (stable IDs, safe to hardcode)
# CachingDisabled: 4135ea2d-6df8-44a3-9df3-4b5a84be39ad
# AllViewerExceptHostHeader: b689b0a8-53d0-40ab-baf2-68738e2966ac
#   (forwards all headers/cookies/query strings except Host, which CloudFront
#    rewrites to the Lambda URL origin — required for Lambda Function URLs)

cloudfront_domain = launcher_url.function_url.apply(
    lambda url: url.removeprefix("https://").rstrip("/")
)

distribution = aws.cloudfront.Distribution(
    "games-distribution",
    aliases=[custom_domain_hostname] if enable_custom_domain and custom_domain_hostname else [],
    enabled=True,
    http_version="http2and3",
    origins=[
        aws.cloudfront.DistributionOriginArgs(
            origin_id="launcher",
            domain_name=cloudfront_domain,
            custom_origin_config=aws.cloudfront.DistributionOriginCustomOriginConfigArgs(
                http_port=80,
                https_port=443,
                origin_protocol_policy="https-only",
                origin_ssl_protocols=["TLSv1.2"],
                # 60s is the CloudFront max without a limit-increase request;
                # long enough for Lambda's 120s timeout and SSE connections.
                origin_read_timeout=60,
                origin_keepalive_timeout=60,
            ),
        )
    ],
    default_cache_behavior=aws.cloudfront.DistributionDefaultCacheBehaviorArgs(
        target_origin_id="launcher",
        viewer_protocol_policy="redirect-to-https",
        allowed_methods=["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"],
        cached_methods=["GET", "HEAD"],
        # CachingDisabled — nothing is cached; every request goes to Lambda.
        cache_policy_id="4135ea2d-6df8-44a3-9df3-4b5a84be39ad",
        # AllViewerExceptHostHeader — forwards all headers/cookies/query strings.
        # CloudFront must rewrite the Host header to the Lambda URL origin domain,
        # otherwise Lambda Function URLs reject the request.
        origin_request_policy_id="b689b0a8-53d0-40ab-baf2-68738e2966ac",
        # Compression must be off — CloudFront buffers the full response before
        # compressing, which breaks SSE streaming.
        compress=False,
    ),
    restrictions=aws.cloudfront.DistributionRestrictionsArgs(
        geo_restriction=aws.cloudfront.DistributionRestrictionsGeoRestrictionArgs(
            restriction_type="none",
        )
    ),
    viewer_certificate=aws.cloudfront.DistributionViewerCertificateArgs(
        acm_certificate_arn=cert.arn if enable_custom_domain and cert else None,
        cloudfront_default_certificate=None if enable_custom_domain else True,
        ssl_support_method="sni-only" if enable_custom_domain else None,
        minimum_protocol_version="TLSv1.2_2021" if enable_custom_domain else None,
    ),
    opts=pulumi.ResourceOptions(provider=acm_provider),
)

pulumi.export("prod_url", launcher_url.function_url)
pulumi.export("games_url", distribution.domain_name)
pulumi.export("games_hostname", custom_domain_hostname)
pulumi.export("budget_name", monthly_budget.name)
pulumi.export("cert_validation_cname", cert.domain_validation_options if cert else None)
