#!/bin/bash

PREFIX=dev
cat <<EOF
global project capitan
#
# General redis container
#
redis image redis:latest
redis hostname ${PREFIX}_redis
redis hook after.run sleep 1
redis hook after.start sleep 1
redis scale 1

#
# General mongodb container
#
mongo image mongo:latest
mongo command mongod --smallfiles
mongo hostname ${PREFIX}_mongo

#
# General nats container
#
nats image nats:latest
nats hostname ${PREFIX}_nats
nats scale 1

#
# Dummy
#
app build ./
app hostname ${PREFIX}_app
app scale 3
app link mongo:mgo
app link something:sth
app volumes-from nats
app volumes-from something

EOF
