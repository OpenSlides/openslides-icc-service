#!/bin/sh

if [ ! -z $dev   ]; then CompileDaemon -log-prefix=false -build="go build -o icc-service ./openslides-icc-service" -command="./icc-service"; fi