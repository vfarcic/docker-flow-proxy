#! /bin/sh


exec telegraf --config /telegraf.conf &

exec docker-flow-proxy "$@"

