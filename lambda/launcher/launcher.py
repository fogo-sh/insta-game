import json
import os
import urllib.request
from time import sleep

import boto3

REGION_NAME = "ca-central-1"
CLUSTER = os.environ.get("ECS_CLUSTER", "insta-game-cluster")
MAX_WAIT_PERIOD = 60

SIDECAR_TOKEN = os.environ.get("SIDECAR_TOKEN", "abc123")

GAMES = json.loads(
    os.environ.get(
        "GAMES",
        json.dumps(
            {
                "xonotic": {
                    "service_name": "instagame-xonotic-service",
                    "sidecar_port": 5001,
                }
            }
        ),
    )
)


def handler(event, context):
    params = event.get("queryStringParameters") or {}
    game_name = params.get("game", "xonotic")
    operation = params.get("operation")
    config_url = params.get("config_url")

    if game_name not in GAMES:
        return {"error": f"Unknown game: {game_name}"}

    config = GAMES[game_name]

    try:
        if operation == "start":
            return run_game(config, config_url=config_url)
        elif operation == "stop":
            return kill_existing_game(config)
        else:
            return get_game_state(config)
    except Exception:
        return get_game_state(config)


def change_desired_count(service_name, desired_count):
    ecs = boto3.client("ecs", region_name=REGION_NAME)
    ecs.update_service(
        cluster=CLUSTER, service=service_name, desiredCount=desired_count
    )


def run_game(config, config_url=None):
    kill_existing_game(config)
    change_desired_count(config["service_name"], 1)
    state = wait_for_game_state(config, "online")

    if config_url and state.get("status") == "online":
        restart_game_with_config(state["public_ip"], config["sidecar_port"], config_url)
        state = wait_for_game_state(config, "online")
        state["config_url"] = config_url

    return state


def kill_existing_game(config):
    change_desired_count(config["service_name"], 0)
    return wait_for_game_state(config, "offline")


def wait_for_game_state(config, desired_status):
    state = get_game_state(config)

    for i in range(10):
        if state["status"] == desired_status:
            return state
        sleep(min(5, MAX_WAIT_PERIOD // 10))
        state = get_game_state(config)

    return state


def get_sidecar_status(public_ip, sidecar_port):
    try:
        req = urllib.request.Request(f"http://{public_ip}:{sidecar_port}/status")
        with urllib.request.urlopen(req, timeout=5) as response:
            return json.loads(response.read())
    except Exception:
        return None


def restart_game_with_config(public_ip, sidecar_port, config_url):
    body = json.dumps({"config_url": config_url}).encode("utf-8")
    req = urllib.request.Request(
        f"http://{public_ip}:{sidecar_port}/restart",
        data=body,
        method="POST",
        headers={
            "Authorization": f"Bearer {SIDECAR_TOKEN}",
            "Content-Type": "application/json",
        },
    )
    with urllib.request.urlopen(req, timeout=10):
        return


def get_game_state(config):
    ecs = boto3.client("ecs", region_name=REGION_NAME)
    ec2 = boto3.client("ec2", region_name=REGION_NAME)

    try:
        tasks = ecs.list_tasks(cluster=CLUSTER, serviceName=config["service_name"])
        task_arn = tasks["taskArns"][0]
        task_info = ecs.describe_tasks(cluster=CLUSTER, tasks=[task_arn])
        network_interface_id = task_info["tasks"][0]["attachments"][0]["details"][1][
            "value"
        ]
        network_interface = ec2.describe_network_interfaces(
            NetworkInterfaceIds=[network_interface_id]
        )
        public_ip = network_interface["NetworkInterfaces"][0]["Association"]["PublicIp"]

        state = {"status": "starting", "public_ip": public_ip, "players": 0}

        sidecar_data = get_sidecar_status(public_ip, config["sidecar_port"])
        if sidecar_data:
            state["players"] = sidecar_data.get("players", 0)
            state["ready"] = sidecar_data.get("ready", False)
            if sidecar_data.get("running") and sidecar_data.get("ready"):
                state["status"] = "online"
        else:
            state["ready"] = False

        return state

    except Exception:
        return {"status": "offline", "players": 0, "ready": False}
