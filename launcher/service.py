import os
from time import sleep

import boto3
import requests
from flask import Flask, render_template
from flask import request

XONOTIC_CONFIG_FILE = 'xonotic_config_file'
CLUSTER = 'insta-game-cluster'
REGION_NAME = 'ca-central-1'

app = Flask(__name__, template_folder='static')


@app.route('/')
def home():
    try:
        response = get_game_state()
    except Exception as e:
        response = str(e)

    return render_template('index.html', response=response)


@app.route('/game', methods=['POST', 'DELETE', 'GET'])
def game():
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


@app.route('/config', methods=['GET', 'POST'])
def get_config():
    if request.method == 'GET':
        with open(XONOTIC_CONFIG_FILE, 'rb') as f:
            return f.read(-1)

    if request.method == 'POST':
        swap_out_config_file(request.form['config'])
        return ''


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

    eni = task_info['tasks'][0]['attachments'][0]['details'][1]['value']
    log(eni)

    neti = ec2.describe_network_interfaces(NetworkInterfaceIds=[eni])
    log(neti)

    return {'status': 'online', 'public_ip': neti['NetworkInterfaces'][0]['Association']['PublicIp']}


def trigger_restart():
    # TODO: tell container to restart game

    pass


def swap_out_config_file(config_file):
    with open(XONOTIC_CONFIG_FILE, 'wb') as f:
        f.write(requests.get(config_file).content)
    if get_game_state()['state'] == 'online':
        trigger_restart()


def log(msg):
    print(f'DEBUG: {msg}')


if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    app.run(debug=True, host='0.0.0.0', port=port)
