import os
from time import sleep
from inspect import getframeinfo, stack
import boto3
import requests
from flask import Flask, render_template
from flask import request

REGION_NAME = 'ca-central-1'
CLUSTER = 'insta-game-cluster'

DEBUG = True
DEFAULT_PORT = 5000

app = Flask(__name__, template_folder='static')


@app.route('/')
def home():
    try:
        response = get_game_state()
    except Exception as e:
        response = str(e)

    return render_template('index.html', response=response)


@app.route('/game', methods=['POST', 'DELETE', 'GET'])
def game_route():
    try:
        if request.method == 'POST':
            return run_game()

        if request.method == 'DELETE':
            return kill_existing_game()

        if request.method == 'GET':
            return get_game_state()

    except Exception as e:
        log(str(e))
        return str(e)


def run_game():
    kill_existing_game()
    change_desired_count(1)
    return wait_for_game_state('online', 60)


def kill_existing_game():
    change_desired_count(0)
    return wait_for_game_state('offline', 60)


def change_desired_count(desired_count):
    ecs = boto3.client('ecs', region_name=REGION_NAME)
    services = ecs.list_services(cluster=CLUSTER)
    service = services['serviceArns'][0]
    ecs.update_service(cluster=CLUSTER, service=service, desiredCount=desired_count)


def wait_for_game_state(desired_status, max_wait_period):
    state = get_game_state()

    for i in range(10):
        if state['status'] == desired_status:
            return state
        else:
            sleep(min(5, max_wait_period // 10))
            state = get_game_state()

    return state


def get_game_state():
    ecs = boto3.client('ecs', region_name=REGION_NAME)
    ec2 = boto3.client('ec2', region_name=REGION_NAME)

    tasks = ecs.list_tasks(cluster=CLUSTER)
    log(tasks)

    if len(tasks['taskArns']) == 0:
        return {'status': 'offline'}

    task_arn = tasks['taskArns'][0]
    log(task_arn)

    task_info = ecs.describe_tasks(cluster=CLUSTER, tasks=[task_arn])
    log(task_info)

    network_interfaces = task_info['tasks'][0]['attachments'][0]['details'][1]['value']
    log(network_interfaces)

    network_interface = ec2.describe_network_interfaces(NetworkInterfaceIds=[network_interfaces])
    log(network_interface)

    network_interface0 = network_interface['NetworkInterfaces'][0]
    if 'Association' not in network_interface0:
        return {'status': 'offline'}

    public_ip = network_interface0['Association']['PublicIp']
    log(public_ip)

    return {'status': 'online', 'public_ip': public_ip}


def log(msg):
    if DEBUG:
        caller = getframeinfo(stack()[1][0])
        print("%s:%d - %s\n" % (caller.filename, caller.lineno, msg))


if __name__ == '__main__':
    port = int(os.environ.get('PORT', DEFAULT_PORT))
    app.run(debug=DEBUG, host='0.0.0.0', port=port)
