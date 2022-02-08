import os
import signal
import subprocess
from time import sleep

import requests
from flask import Flask

CONFIG_URL = os.environ.get('CONFIG_URL', 'https://raw.githubusercontent.com/xonotic/xonotic/master/server/server.cfg')

DEBUG = True
DEFAULT_PORT = 5001

app = Flask(__name__)


@app.route('/')
def home():
    return {"hi": "hi"}


@app.route('/restart')
def home():
    get_config_and_start_game()
    return {"hi": "hi"}


def log(msg):
    if DEBUG:
        print(f'DEBUG: {msg}')


def get_config_and_start_game():
    with open('/Xonotic/server/server.cfg', 'wb') as f:
        f.write(requests.get(CONFIG_URL).content)
    game_pid = int(subprocess.check_output(["pidof", "./xonotic-linux64-dedicated"]))
    os.kill(game_pid, signal.SIGKILL)
    sleep(5)
    subprocess.Popen(['./xonotic-linux-dedicated.sh'])


if __name__ == '__main__':
    port = int(os.environ.get('PORT', DEFAULT_PORT))
    get_config_and_start_game()

    app.run(debug=DEBUG, host='0.0.0.0', port=port)
