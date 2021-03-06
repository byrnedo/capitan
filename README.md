# Capitan

[![GoDoc](https://godoc.org/github.com/byrnedo/capitan?status.svg)](https://godoc.org/github.com/byrnedo/capitan)

Capitan is a tool for managing multiple Docker containers based largely on [crowdr](https://github.com/polonskiy/crowdr)

Capitan is only a wrapper around the docker cli tool, no api usage whatsoever (well... an `inspect` command here and there).
This means it will basically work with all versions of docker.

    $ capitan up

    Run arguments changed, doing blue-green redeploy: capitan_redis_green_1
    Running capitan_redis_blue_1
    cd939956f332391489d0383610d9da2c420595d495934c3221376dbf68854316
    Removing old container capitan_redis_green_1...
    Already running capitan_mongo_blue_1
    Already running capitan_nats_blue_1
    Already running capitan_app_blue_1

## Features

- Provides commands which operate on collection of containers.
- Uses predefined description of containers from readable configuration file.
- Can use any docker run option that is provided by your docker version.
- The order of starting containers is defined in configuration file.
- The order of stopping containers is the reverse of the order of starting.
- Easy to install, compiled static go binaries available for linux, and mac. It doesn't require any execution environment or other libraries.
- Allows the use of bash as hooks for many capitan commands.
- [Blue/Green](https://docs.cloudfoundry.org/devguide/deploy-apps/blue-green.html) deployment - option to only remove original container when starting new version of same container if it starts and passes hook commands.


## Installation

Head over to the [releases](https://github.com/byrnedo/capitan/releases) page to download a pre-build binary or deb file.

Or using go:

    go get github.com/byrnedo/capitan

## Commands

### Invasive commands

#### `up`
Create then run or update containers 
Recreates if:

1. ~~If newer image is found it will remove the old container and run a new one~~ No longer does this as capitan can't know which node to check images for when talking to a swarm.
2. Container config has changed
    
Starts stopped containers

    capitan up
    # Optionally can attach to output using `--attach|-a` flag.
    capitan up -a

#### `create`
Create but don't run containers

    capitan create
    
#### `start`
Start stopped containers

    capitan start
    # Optionally can attach to output using `--attach|-a` flag.
    capitan start -a
    
#### `scale`
Start or stop instances of a container until required amount are running

    # run 5 instances of mysql
    capitan scale mysql 5
    
NOTE: for containers started via this command to be accepted by further commands, the config output must be altered to state the required instances

##### `restart`	
Restart containers
    
    capitan restart
    # Further arguments passed through to docker, example `capitan start -t 5`
    capitan restart -t 10

##### `stop`	
Stop running containers
    
    capitan stop
    # Further arguments passed through to docker, example `capitan stop -t 5`
    capitan stop -t 10
    
##### `kill`	
Kill running containers using SIGKILL or a specified signal
    
    capitan kill
    # Further arguments passed through to docker, example `capitan kill --signal KILL`
    capitan kill --signal KILL

##### `rm`		
Remove stopped containers
    
    capitan rm
    # Further arguments passed through to docker, example `capitan rm -f`
    capitan rm -fv
    
### Non invasive commands
    
##### `ps`
Show container status
    
    - Further arguments passed through to docker, example `capitan ps -a`

##### `ip`
Show container ip addresses

##### `logs`
Follow container logs

##### `pull`
Pull images for all containers

##### `build`
Build any containers with 'build' flag set (WIP)


## Configuration

     
### Global options

     --cmd, -c "./capitan.cfg.sh"	Command used to obtain config
     --debug, -d				    Print extra log messages
     --dry-run, --dry			    Preview outcome, no changes will be made
     --filter, -f 		            Filter to run action on a specific container only
     --help, -h				        Show help
     --version, -v			        Print the version

### Config file/output

Service config is read from stdout of the command defined with `--cmd` .

`capitan` by default runs the command `./capitan.cfg.sh` in the current directory to get the config. This can be customized with `--cmd|-c` flag.

    capitan --cmd ./someotherexecutable <some action>

You could use any command which generates a valid config. It doesn't have to be a bash script like in the example or default.

### Filtering

A single service type can specified for an action by using the `--filter|-f` flag. So if your conf looked like this:

    ...
    fooapp hostname blah
    ...
    
You could filter any command to run on just that service type by doing:

    capitan --filter fooapp <some action>

#### Global options

##### `global project`
The project name, defaults to current working directory

##### `global project_sep`
String to use to create container name from `project` and name specified in config

##### `global blue_green [true/false]`
String to deploy using blue/green handover. Defaults to false. This can be turned on/off per container with

    CONTAINER_NAME blue-green [true/false]
    
#### `global hook [hook name] [hook command]`
Allows for a custom shell command to be evaluated once at the following points:

- Before/After up (`before.up`, `after.up`)
    - This occurs during the `up` command
- Before/After Start (`before.start`, `after.start`)
    - This will occur in the `start` command
- Before/After Stop (`before.stop`, `after.stop`)
    - This will occur in the `stop` command
- Before/After Kill (`before.kill`, `after.kill`)
    - This will occur in the `kill` command
- Before/After Rm (`before.rm`, `after.rm`)
    - This will occur in the `rm` command

#### Container Options

The output format must be:

    CONTAINER_NAME COMMAND [ARGS...]
 
All commands are passed through to docker cli as `--COMMAND` EXCEPT the following:

#### `build`
This allows a path to be given for a dockerfile. Note, it will attempt to build every time. Use `build-args` and pass `--no-cache` to force a full clean build each time.

#### `build-args`
Any further arguments that need to be passed when building.

#### `hook [hook name] [hook command]`
Allows for a custom shell command to be evaluated at the following points **for each container**

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

#### `rm`

By default capitan runs all commands with `-d`. This flag makes capitan run the command with `-rm` instead.

WARNING: This feature is experimental and may result in unexpected failures. A more predictable way is to leverage `docker wait` along with a dynamic label.
For example:

    mycontainer label $(date +%s)
    mycontainer hook after.run docker wait \$CAPITAN_CONTAINER_NAME

#### `volumes-from`

An attempt to resolve a volume-from arg to the first instance of a container is made. Otherwise the unresolved name is used.

WARNING: When scaling, if the container name resolves to a container defined in capitan's config, it will always resolve to the first instance.
For example: `app volumes-from mycontainer` will always resolve to `<project>_mycontainer_1`

### Environment Variables 

The following environment variables are available when creating the containers and when running **container hooks**

    # container name
    CAPITAN_CONTAINER_NAME
    # container type 
    CAPITAN_CONTAINER_SERVICE_TYPE
    # instance of this type,eg if you have scale = 5 then each container will have their own instance number from 1 -> 5
    CAPITAN_CONTAINER_INSTANCE_NUMBER
    # the project name
    CAPITAN_PROJECT_NAME
    
The following environment variables are available to all **hook** scripts

    CAPITAN_PROJECT_NAME
    CAPITAN_HOOK_NAME
    

For example, following `capitan.cfg.sh`

    #!/bin/bash

    cat <<EOF
    global project test

    mysql name mymysql
    mysql label containerName=\$CAPITAN_CONTAINER_NAME
    mysql label containerServiceName=\$CAPITAN_CONTAINER_SERVICE_NAME
    mysql label containerInstanceNumber=\$CAPITAN_CONTAINER_INSTANCE_NUMBER
    mysql label projectName=\$CAPITAN_PROJECT_NAME
    mysql hook after.run echo "hook: \$CAPITAN_HOOK_NAME: ran \$CAPITAN_CONTAINER_NAME in project \$CAPITAN_PROJECT_NAME"
    EOF

Would result in the following run command:

    docker run -d --name test_mysql_1 
        --label containerName=test_mysql_1 
        --label containerServiceName=mysql
        --label containerInstanceNumber=1
        --label projectName=test

And the following hook ouput

    Running test_mysql_1
    34e7fffb937c3154c2a963ee605c7958404aa5d80519db4ef3d2a80a06974021
    hook: after.run: ran test_mysql_1 in project test

Note that the `$` must be escaped if using HEREDOC or double quotes in bash.


### Example Config

Check out [dev-stack](https://github.com/byrnedo/dev-stack) for an example. 
Clone it and just run `capitan --dry up`.

Or check out the `./example` dir.

Also, here's something to whet your appetite:
    
    #!/bin/bash
    PREFIX=dev
    
    cat <<EOF
    #
    # General redis container
    #
    redis image redis:latest
    redis hostname ${PREFIX}_redis
    redis publish 6379
    redis hook after.run echo "look everyone, I ran \$CAPITAN_CONTAINER_NAME" && sleep 3
    redis hook after.start sleep 3
    
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
