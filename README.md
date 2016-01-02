# Capitan

Capitan is a tool for managing multiple Docker containers based largely on [crowdr](https://github.com/polonskiy/crowdr)

Capitan is only a wrapper around the docker cli tool, no api usage whatsoever.
This means it will basically work with all versions of docker.

## Features

- Bash Support
- Hooks
- Predictable run sequence
- Future proof

## Commands

- `capitan up`		Create then run or update containers 
    - Recreates if:
        1. If newer image is found it will remove the old container and run a new one
        2. Container config has changed
    - Starts stopped containers
    
- `capitan ps`		Show container status

- `capitan ip`		Show container ip addresses

- `capitan build`   Build any containers with 'build' flag set (WIP)

- `capitan start`   Start stopped containers

- `capitan restart`	Restart containers

- `capitan stop`	Stop running containers

- `capitan kill`	Kill running containers using SIGKILL or a specified signal

- `capitan rm`		Remove stopped containers
     
## Global options
     --file, -f "./capitan.cfg.sh"	config file to read
     --debug, -d				print extra log messages
     --dry-run, --dry			preview outcome, no changes will be made
     --help, -h				show help
     --version, -v			print the version
 
## Config file

Services are described in the config file which is plane old bash. The config is then read from stdout.

The format is:

    CONTAINER_NAME COMMAND ARGS
 
All commands are passed through to docker cli as `--COMMAND` EXCEPT the following:

- `build`: This allows a path to be given for a dockerfile.

- `hook`: Allows for a custom shell command to be evaluated at the following points:

    - Before/After Run (`before.run`, `after.run`)
    - Before/After Start (`before.run`, `after.run`)
    - Before/After Stop (`before.run`, `after.run`)
    - Before/After Kill (`before.run`, `after.run`)
    - Before/After Rm (`before.run`, `after.run`)

- `global project`: The project name, defaults to current working directory

- `global project_sep`: String to use to create container name from `project` and name specified in config


### Example Config :
    
    #!/bin/bash
    PREFIX=dev
    
    cat <<EOF
    #
    # General redis container
    #
    redis image redis:latest
    redis hostname ${PREFIX}_redis
    redis publish 6379
    redis hook after.run sleep 10
    
    #
    # General mongodb container
    #
    mongo image mongo:latest
    mongo command mongod --smallfiles
    mongo hostname ${PREFIX}_mongo
    mongo publish 27017
    
    #
    # General nats container
    #
    nats image nats:latest
    nats hostname ${PREFIX}_nats
    nats publish 4222
    EOF

