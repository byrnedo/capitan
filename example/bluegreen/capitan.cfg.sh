#!/bin/bash

PREFIX=dev
cat <<EOF
# Set project name to 'capitan'
global project capitan
global blue_green true

# --------------------------------------------------
# General redis container
# --------------------------------------------------
#
redis image redis:latest
redis hostname ${PREFIX}_rediss

# sleep one second after 'run' command
redis hook after.run sleep 3

# sleep one second after 'start' command
redis hook after.start sleep 1
redis scale 1

# --------------------------------------------------
# General mongodb container
# --------------------------------------------------
#
mongo image mongo:latest
mongo command mongod --smallfiles
mongo hostname ${PREFIX}_mongo
mongo env test=\$CAPITAN_INSTANCE_NUMBER

# --------------------------------------------------
# General nats container
# --------------------------------------------------
#
nats image nats:latest
nats hostname ${PREFIX}_nats
nats scale 1

# --------------------------------------------------
# Dummy
# --------------------------------------------------
#
# Builds dockerfile locate at ./ and tags it as 'capitan_app', container will then use this image

app build ../
app hostname ${PREFIX}_app

# run 3 instances
app scale 3


EOF
