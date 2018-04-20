#!/bin/bash

function getContainerId {
    echo $(docker ps --format "{{.ID}}\t{{.Image}}" | grep jupyter_jupyter | awk '{print $1}')
}

containerId=$(getContainerId)
if [[ -z $containerId ]]; then
    docker-compose up -d
    containerId=$(getContainerId)
fi

sleep 1
url=$(docker exec $containerId jupyter notebook list | grep http | awk '{print $1}')
if [[ -z $url ]]; then
    echo Cannot determine url
    exit 1
fi

if [[ "$OSTYPE" == "linux-gnu" ]]; then
    xdg-open $url
elif [[ "$OSTYPE" == "darwin"* ]]; then
    open $url
else
    echo $url
fi
