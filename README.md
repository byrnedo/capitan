# Capitan

Capitan is a tool for managing multiple Docker containers based largely on [crowdr](https://github.com/polonskiy/crowdr)

Capitan is only a wrapper around the docker cli tool, no api usage whatsoever.
This means it will basically work with all versions of docker.

## Why not docker-compose?

1. lack of variables
2. options support is always behind actual docker version
3. up restarts containers in wrong order
4. no hooks system
5. hasslefree replacement for non x86/x86_64 architectures

## Why not crowdr?

Written totally in bash. I love bash but I personally feel safer with go. That is all.

## Capitan Features

1. Shell Support - Config is read from stdout of a shell command. Extremely flexible
2. Hooks - hooks for before and after every intrusive action
3. Predictable run sequence - containers started in order defined
4. Future proof - options are passed through on most commands to docker cli, very simple.

## Commands

### Invasive commands

- `capitan up`		Create then run or update containers 
    - Recreates if:
        1. If newer image is found it will remove the old container and run a new one
        2. Container config has changed
    - Starts stopped containers

    - Optionally can attach to output using `--attach|-a` flag.
    
- `capitan start`   Start stopped containers

    - Optionally can attach to output using `--attach|-a` flag.

- `capitan restart`	Restart containers
    
    - Further arguments passed through to docker, example `capitan start -t 5`

- `capitan stop`	Stop running containers
    
    - Further arguments passed through to docker, example `capitan stop -t 5`
    
- `capitan kill`	Kill running containers using SIGKILL or a specified signal
    
    - Further arguments passed through to docker, example `capitan kill --signal KILL`

- `capitan rm`		Remove stopped containers
    
    - Further arguments passed through to docker, example `capitan rm -f`
    
### Non invasive commands
    
- `capitan ps`		Show container status
    
    - Further arguments passed through to docker, example `capitan ps -a`

- `capitan ip`		Show container ip addresses

- `capitan logs`    Follow container logs

- `capitan pull`    Pull images for all containers

- `capitan build`   Build any containers with 'build' flag set (WIP)

     
## Global options
     --cmd, -c "./capitan.cfg.sh"	command used to obtain config
     --debug, -d				    print extra log messages
     --dry-run, --dry			    preview outcome, no changes will be made
     --help, -h				        show help
     --version, -v			        print the version
 
## Configuration

Services are described in the config output which is read from stdout.
You could use any command which generates a valid config. It doesn't have to be a bash script like in the example.

`capitan` by default runs the command `./capitan.cfg.sh` in the current directory to get the config. This can be customized with `-c` flag.

The output format is:

    CONTAINER_NAME COMMAND ARGS
 
All commands are passed through to docker cli as `--COMMAND` EXCEPT the following:

#### `build`
This allows a path to be given for a dockerfile.

#### `hook`
Allows for a custom shell command to be evaluated at the following points:

- Before/After Run (`before.run`, `after.run`)
    - This occurs during the `up` command
- Before/After Start (`before.start`, `after.start`)
    - This will occur in the `up`, `start` and `restart` command
- Before/After Stop (`before.stop`, `after.stop`)
    - This will occur in the `stop` command only
- Before/After Kill (`before.kill`, `after.kill`)
    - This will occur in the `kill` command only
- Before/After Rm (`before.rm`, `after.rm`)
    - This will occur in the `up` and `rm` command
       
*NOTE* hooks do not conform exactly to each command. Example: an `up` command may `rm` and then `run` a container OR just `start` a stopped container.
    

####`global project`
The project name, defaults to current working directory

#### `global project_sep`
String to use to create container name from `project` and name specified in config


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
    redis hook after.start sleep 10
    
    #
    # General mongodb container
    #
    mongo image mongo:latest
    mongo command mongod --smallfiles
    mongo hostname ${PREFIX}_mongo
    mongo publish 27017
    
    #
    # My app
    #
    app build ./
    app publish 80
    app link redis
    app link mongo:mongodb
    EOF

## Roadmap

1. Tests
2. Scaling (multiple instances of same container)
3. More efficient `up` logic
4. Helpful aliases in shell env.
5. More flexible `build` command
