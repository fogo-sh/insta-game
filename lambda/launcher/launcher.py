from time import sleep

import boto3

REGION_NAME = 'ca-central-1'
CLUSTER = 'insta-game-cluster'
DEBUG = True
MAX_WAIT_PERIOD = 60


def handler(event, context):
    try:
        operation = event['queryStringParameters']['operation']
        if operation == 'start':
            return run_game()
        elif operation == 'stop':
            return kill_existing_game()
    except Exception:
        return get_game_state()


def change_desired_count(desired_count):
    ecs = boto3.client('ecs', region_name=REGION_NAME)
    services = ecs.list_services(cluster=CLUSTER)
    service = services['serviceArns'][0]
    ecs.update_service(cluster=CLUSTER, service=service, desiredCount=desired_count)


def run_game():
    kill_existing_game()
    change_desired_count(1)
    return wait_for_game_state('online')


def kill_existing_game():
    change_desired_count(0)
    return wait_for_game_state('offline', )


def wait_for_game_state(desired_status):
    state = get_game_state()

    for i in range(10):
        if state['status'] == desired_status:
            return state
        else:
            sleep(min(5, MAX_WAIT_PERIOD // 10))
            state = get_game_state()

    return state


def get_game_state():
    ecs = boto3.client('ecs', region_name=REGION_NAME)
    ec2 = boto3.client('ec2', region_name=REGION_NAME)

    try:
        tasks = ecs.list_tasks(cluster=CLUSTER)
        task_arn = tasks['taskArns'][0]
        task_info = ecs.describe_tasks(cluster=CLUSTER, tasks=[task_arn])
        network_interfaces = task_info['tasks'][0]['attachments'][0]['details'][1]['value']
        network_interface = ec2.describe_network_interfaces(NetworkInterfaceIds=[network_interfaces])
        network_interface0 = network_interface['NetworkInterfaces'][0]
        public_ip = network_interface0['Association']['PublicIp']
        return {'status': 'online', 'public_ip': public_ip}

    except Exception:
        return {'status': 'offline'}
