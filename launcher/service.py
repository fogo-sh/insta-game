import os
import boto3

from flask import Flask, render_template
from flask import request

app = Flask(__name__, template_folder='static')


@app.route('/')
def home():
    try:
        response = get_game_state()
    except Exception as e:
        response = str(e)

    return render_template('index.html', response=response)


def kill_existing_game():
    print('Killing existing game')
    pass


def run_game(game, config_file):
    print(f'Running {game} with {config_file} config')
    pass


@app.route('/game', methods=['POST', 'DELETE'])
def game():
    try:

        if request.method == 'POST':
            kill_existing_game()
            run_game(request.form['game'], request.form['config_file'])
            return 'initiated'

        if request.method == 'DELETE':
            kill_existing_game()
            return 'deleted'

        if request.method == 'GET':
            return get_game_state()

    except Exception as e:
        log(str(e))
        return str(e)


def get_game_state():
    ecs = boto3.client('ecs', region_name='ca-central-1')
    ec2 = boto3.client('ec2', region_name='ca-central-1')
    tasks = ecs.list_tasks(cluster='insta-game-cluster')
    log(tasks)

    if len(tasks['taskArns']) == 0:
        return {'status': 'offline'}

    task_arn = tasks['taskArns'][0]
    log(task_arn)

    task_info = ecs.describe_tasks(cluster='insta-game-cluster', tasks=[task_arn])
    log(task_info)

    eni = task_info['tasks'][0]['attachments'][0]['details'][1]['value']
    log(eni)

    neti = ec2.describe_network_interfaces(NetworkInterfaceIds=[eni])
    log(neti)

    return {'status': 'online', 'public_ip': neti['NetworkInterfaces'][0]['Association']['PublicIp']}


def log(msg):
    print(f'DEBUG: {msg}')


if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    app.run(debug=True, host='0.0.0.0', port=port)
