#!/usr/bin/env bash
CONTAINER=insta-game-launcher
POD=${CONTAINER}pod
podman container stop $CONTAINER
podman container rm $CONTAINER
podman pod rm $POD
podman pod create --name $POD --publish=5000:5000
podman build --format docker --no-cache -t $CONTAINER:latest .
podman run -dit --name=$CONTAINER --pod=$POD $CONTAINER:latest
curl -X POST -F 'game=xonotic' -F 'config_file=https://raw.githubusercontent.com/xonotic/xonotic/master/server/server.cfg' http://127.0.0.1:5000/game
curl -X DELETE http://127.0.0.1:5000/game
