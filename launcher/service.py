import os

from flask import Flask, render_template
from flask import request

app = Flask(__name__, template_folder='static')


@app.route('/')
def home():
    return render_template('index.html')


@app.route('/game', methods=['POST', 'DELETE'])
def game():
    if request.method == 'POST':
        data = request.form
        return f"{data['game']} {data['config_file']}\n"

    if request.method == 'DELETE':
        return "deleted\n"


if __name__ == "__main__":
    port = int(os.environ.get('PORT', 5000))
    app.run(debug=True, host='0.0.0.0', port=port)
