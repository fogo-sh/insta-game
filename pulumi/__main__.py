import ipaddress
import json

import pulumi
import pulumi_aws as aws

from game_service import GameService

config = pulumi.Config()
sidecar_token = config.require_secret("sidecarToken")
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
default_data_url = config.get("defaultDataUrl")
xonotic_data_url = config.get("xonoticDataUrl") or default_data_url
qss_m_data_url = config.get("qssmDataUrl") or default_data_url
q2repro_data_url = config.get("q2reproDataUrl") or default_data_url
bzflag_data_url = config.get("bzflagDataUrl") or default_data_url
ut99_data_url = config.get("ut99DataUrl") or default_data_url
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
        aws.ec2.SecurityGroupIngressArgs(
            from_port=27910,
            to_port=27910,
            protocol="udp",
            cidr_blocks=["0.0.0.0/0"],
        ),
        aws.ec2.SecurityGroupIngressArgs(
            from_port=5154,
            to_port=5154,
            protocol="tcp",
            cidr_blocks=["0.0.0.0/0"],
        ),
        aws.ec2.SecurityGroupIngressArgs(
            from_port=5154,
            to_port=5154,
            protocol="udp",
            cidr_blocks=["0.0.0.0/0"],
        ),
        aws.ec2.SecurityGroupIngressArgs(
            from_port=7777,
            to_port=7781,
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
    cpu=1024,
    memory=2048,
    cpu_architecture="ARM64",
    data_url=q2repro_data_url,
    protocol="quake2",
    game_cmd="./q2proded",
    game_args=(
        "+set dedicated 1 +set basedir /opt +set game baseq2"
        " +set net_ip 0.0.0.0 +set net_port 27910 +set maxclients 4 +exec server.cfg"
    ),
    game_port=27910,
    game_quit_cmd="quit",
    game_quit_timeout=15,
    config_path="/opt/baseq2/server.cfg",
)

bzflag = GameService(
    "bzflag",
    game_name="bzflag",
    name_prefix=regional_name("game"),
    image="ghcr.io/fogo-sh/insta-game:bzflag",
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
    data_url=bzflag_data_url,
    game_port=5154,
    game_port_protocols=["tcp", "udp"],
    protocol="bzflag",
    game_cmd="/usr/games/bzfs",
    game_args="-conf /opt/data/server.cfg -p 5154",
    game_quit_cmd="quit",
    game_quit_timeout=15,
    config_path="/opt/data/server.cfg",
)

ut99 = GameService(
    "ut99",
    game_name="ut99",
    name_prefix=regional_name("game"),
    image="ghcr.io/fogo-sh/insta-game:ut99",
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
    data_url=ut99_data_url,
    game_port=7777,
    extra_port_mappings=[
        {"containerPort": 7778, "protocol": "udp"},
        {"containerPort": 7779, "protocol": "udp"},
        {"containerPort": 7780, "protocol": "udp"},
        {"containerPort": 7781, "protocol": "udp"},
    ],
    protocol="ut99",
    game_cmd="/usr/local/bin/start-ut99.sh",
    game_args=(
        "/opt/SystemARM64/ucc-bin-arm64 server DM-Deck16][?game=Botpack.DeathMatchPlus"
        " ini=/opt/data/UnrealTournament.ini -nohomedir"
    ),
    game_quit_cmd="exit",
    game_quit_timeout=15,
    config_path="/opt/data/UnrealTournament.ini",
)

# ---- Lambda ----

launcher = aws.lambda_.Function(
    "launcher",
    name=regional_name("launcher"),
    runtime="nodejs22.x",
    handler="index.handler",
    timeout=120,
    role=lambda_role.arn,
    code=pulumi.FileArchive("../launcher/dist"),
    environment=aws.lambda_.FunctionEnvironmentArgs(
        variables=pulumi.Output.all(
            sidecar_token,
            xonotic.service_name,
            cluster.name,
            qssm.service_name,
            q2repro.service_name,
            web_ui_passphrase,
            api_token,
            discord_public_key,
            discord_bot_token,
            discord_app_id,
            bzflag.service_name,
            ut99.service_name,
        ).apply(
            lambda args: {
                "SIDECAR_TOKEN": args[0],
                "ECS_CLUSTER": args[2],
                "WEB_UI_PASSPHRASE": args[5],
                "API_TOKEN": args[6],
                "DISCORD_PUBLIC_KEY": args[7],
                "DISCORD_BOT_TOKEN": args[8],
                "DISCORD_APP_ID": args[9],
                "GAMES": json.dumps(
                    {
                        "xonotic": {
                            "serviceName": args[1],
                            "sidecarPort": 5001,
                        },
                        "qssm": {
                            "serviceName": args[3],
                            "sidecarPort": 5001,
                        },
                        "q2repro": {
                            "serviceName": args[4],
                            "sidecarPort": 5001,
                        },
                        "bzflag": {
                            "serviceName": args[10],
                            "sidecarPort": 5001,
                        },
                        "ut99": {
                            "serviceName": args[11],
                            "sidecarPort": 5001,
                        },
                    }
                ),
            }
        ),
    ),
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
