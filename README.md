# Capitan

[![GoDoc](https://godoc.org/github.com/byrnedo/capitan?status.svg)](https://godoc.org/github.com/byrnedo/capitan)

Capitan is a tool for managing multiple Docker containers based largely on [crowdr](https://github.com/polonskiy/crowdr)

Capitan is only a wrapper around the docker cli tool, no api usage whatsoever (well... an `inspect` command here and there).
This means it will basically work with all versions of docker.

![Capitan showcase]
(output.gif)

## Capitan Features

1. Shell Support - Config is read from stdout of a shell command. Extremely flexible
2. Hooks - hooks for before and after every intrusive action
3. Predictable run sequence - containers started in order defined
4. Future proof - options are passed through on most commands to docker cli, very simple.

## Commands

### Invasive commands

#### `capitan up`
Create then run or update containers 
Recreates if:
    1. If newer image is found it will remove the old container and run a new one
    2. Container config has changed
Starts stopped containers

    capitan up
    # Optionally can attach to output using `--attach|-a` flag.
    capitan up -a

#### `capitan create`
Create but don't run containers

    capitan create
    
#### `capitan start`
Start stopped containers

    capitan start
    # Optionally can attach to output using `--attach|-a` flag.
    capitan start -a
    
#### `capitan scale`
Start or stop instances of a container until required amount are running

    # run 5 instances of mysql
    capitan scale mysql 5
    
NOTE: for containers started via this command to be accepted by further commands, the config output must be altered to state the required instances

##### `capitan restart`	
Restart containers
    
    capitan restart
    # Further arguments passed through to docker, example `capitan start -t 5`
    capitan restart -t 10

##### `capitan stop`	
Stop running containers
    
    capitan stop
    # Further arguments passed through to docker, example `capitan stop -t 5`
    capitan stop -t 10
    
##### `capitan kill`	
Kill running containers using SIGKILL or a specified signal
    
    capitan kill
    # Further arguments passed through to docker, example `capitan kill --signal KILL`
    capitan kill --signal KILL

##### `capitan rm`		
Remove stopped containers
    
    capitan rm
    # Further arguments passed through to docker, example `capitan rm -f`
    capitan rm -fv
    
### Non invasive commands
    
##### `capitan ps`
Show container status
    
    - Further arguments passed through to docker, example `capitan ps -a`

##### `capitan ip`
Show container ip addresses

##### `capitan logs`
Follow container logs

##### `capitan pull`
Pull images for all containers

##### `capitan build`
Build any containers with 'build' flag set (WIP)


## Configuration

     
### Global options
     --cmd, -c "./capitan.cfg.sh"	command used to obtain config
     --debug, -d				    print extra log messages
     --dry-run, --dry			    preview outcome, no changes will be made
     --help, -h				        show help
     --version, -v			        print the version

### Config file/output

Service config is read from stdout of the command defined with `--cmd` .

`capitan` by default runs the command `./capitan.cfg.sh` in the current directory to get the config. This can be customized with `-c` flag.

You could use any command which generates a valid config. It doesn't have to be a bash script like in the example or default.

The output format must be:

    CONTAINER_NAME COMMAND [ARGS...]
 
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

#### `scale`
Number of instances of the container to run. Default is 1.

NOTE: this is untested with links ( I don't use links )

#### `link`
An attempt to resolve a link to the first instance of a container is made. Otherwise the unresolved name is used.

WARNING: When scaling, if the link resolves to a container defined in capitan's config, it will always resolve to the first instance.
For example: `app link mycontainer:some-alias` will always resolve to `<project>_mycontainer_1`

#### `volumes-from`

An attempt to resolve a volume-from arg to the first instance of a container is made. Otherwise the unresolved name is used.

WARNING: When scaling, if the container name resolves to a container defined in capitan's config, it will always resolve to the first instance.
For example: `app volumes-from mycontainer` will always resolve to `<project>_mycontainer_1`

####`global project`
The project name, defaults to current working directory

#### `global project_sep`
String to use to create container name from `project` and name specified in config


### Example Config
    
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
2. More efficient `up` logic
3. Helpful aliases in shell env.
4. More flexible `build` command
