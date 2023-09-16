```

                                            _        _____
                                 /\        | |      |  __ \
                                /  \  _   _| |_ ___ | |  | | _____   __
                               / /\ \| | | | __/ _ \| |  | |/ _ \ \ / /
                              / ____ \ |_| | || (_) | |__| |  __/\ V /
                             /_/    \_\__,_|\__\___/|_____/ \___| \_/
                                · Automate your dev environments! ·

```

A tool that automates the creation of development environments on the cloud, powered by Kubernetes.

# Table of contents
- [Table of contents](#table-of-contents)
- [Motivation](#motivation)
- [Features](#features)
- [Scope](#scope)
- [Usage](#usage)
  - [Setup](#setup)
    - [Development](#development)
    - [Production](#production)
  - [Example usage](#example-usage)
- [License](#license)

# Motivation

During my years of studying and working in the software engineering field, I had
many reaccuring problems that made it harder for me to be as productive as possible.
Among these problems, there was one that seemed particularly hard to solve in a
simple, reliable and replicable way, it's the setup of development environments.

Back in school, I've noticed that this was a real problems because as beginners,
my classmates and I often had problems with the initial setup of our environments
either because we couldn't install the tools we needed (compilers, CLI tools,
databases ...) successfully on our machines because of our lack of familiarity
with these tools, or because our personal machines weren't powerful enough to
work on some types of projects comfortably.

This continued to be a problem I had even after I started my final year internship,
where I often had to work with projects that were either too big, because they
were composed of multiple microservices and databases that I had to run simultaniously
on my local machine, or too old, because they require older versions of certain
runtimes to be executed which made it harder to manage all the versions of these
runtimes locally.

That's when I thought to myself, wouldn't it be great if there was a tool that can
manage all this complexity and make the process of provisionning development
environments simpler and more straightforward? Would that even be possible?
As it turns out the answer to both of these questions is Yes! AutoDev is the
tool that makes it possible to do all of this by providing an abstraction layer
over a set of Kubernetes resources that it creates in the Kubernetes cluster
where it's deployed.

# Features

Using AutoDev, you can create sessions that represent development environments.
Each session is composed of a number of components that represent the development
tools that are needed to work on the project in question. These are processes like
an online IDE, a database, a cache server, a self hosted 3rd partie service ...
Each session has persistant storage where the developer can store code, configuration
files, db data ...The sessions can be stopped or deleted just like most cloud
resources you can manage on public cloud providers.

Session components can be provisioned with as much computing resources as
you will need to work on your dev projects, making it easier than ever to handle
compute intensive workloads without sacrificing productivity.

AutoDev exposes a REST API that makes it possible to manage sessions by sending
HTTP requests. The OpenAPI documentation of AutoDev will soon be released.

The features that distinguishe AutoDev from cloud providers, that are also capable
of creating databases and servers automatically, are its ability to provide browser
based code editors (or IDEs) that are preconfigured with the tools that you requested,
and its simplicity, because it makes the provisioned environment very similar to
what a traditional equivalent local environment would be (with all of the
requested services accessible on localhost).


# Scope

AutoDev does not (and will not) handle the system's users, billing informations,
session quotas...The only things that it manages are the sessions (aka development
environments) and their underlying Kubernetes resources. This means that for
production use cases, it must be deployed either as a microservice inside a
project, or behind an ApiGateway that acts as a middleware for managing these
missing functionalities.


# Usage

## Setup

### Development

By default (on non-production environments), AutoDev will attempt to connect to
a Redis database on localhost:6379. To set it up either install Redis on your
local machine or start it inside a docker container using the following command:

```bash
docker run -d -p 6379:6379 --name myredis redis
```

To override this behavior and provide a custom Redis address, port, and password,
you need to set the following environment variables:

```bash
export AUTODEV_ENV="production"
export AUTODEV_REDIS_ADDR="redis.server.address:port"
export AUTODEV_REDIS_PASSWORD="redispassword"
```

Your must also configure `kubectl` access to a Kubernetes cluster that you want
AutoDev to create resources in. You will need the following permissions in the
`default` namespace of this cluster:
- On `Pods`: `["get", "delete", "list", "create"]`
- On `Services`: `["get", "delete", "create"]`
- On `Ingresses`: `["get", "update"]`

Your can then start the project by running the following command:

```bash
go run ./cmd/main/main.go
```

The REST API will be available on `localhost:8080`.

### Production

There are 2 main ways AutoDev can be deployed in for production environments,
either inside or outside the Kubernetes cluster in which it will be creating
resources. The docs for these 2 deployment methods will soon be added.


## Example usage

Let's say you want to create a development environment that's composed of a code
editor, a Redis database for cache, and a MongoDB database for your application
data. To create this environment using AutoDev you need to follow these steps
(you need to have it running on `localhost:8080`)
- Initialize a new session named `test`:
```bash
curl --location --request POST 'http://localhost:8080/init/test'
```

- Create the components you need in this session:
```bash
curl --location --request POST 'http://localhost:8080/create/test' \
--header 'Content-Type: application/json' \
--data-raw '{
    "components": [
        {
            "componentType": "code",
            "exposeComponent": true,
            "componentID": "my-code-editor",
            "componentMetadata": {
                "Password": ""
            }
        },
        {
            "componentType": "redis",
            "exposeComponent": false,
            "componentID": "my-redis",
            "componentMetadata": {
                "Password": ""
            }
        },
        {
            "componentType": "mongo",
            "exposeComponent": false,
            "componentID": "my-mongo",
            "componentMetadata": {
                "Password": ""
            }
        }
    ]
}'
```

- Fetch the state of your session:
```bash
curl --location --request POST 'http://localhost:8080/refresh/test'
```

The response will look like this:
```json
{
    "message": "session test refreshed successfully",
    "result": {
        "sessionState": "running",
        "components": [
            {
                "componentType": "code",
                "exposeComponent": true,
                "componentID": "my-code-editor",
                "componentMetadata": {
                    "Url": "test.my-code-editor.hamzaboudouche.tech"
                }
            },
            {
                "componentType": "redis",
                "exposeComponent": true,
                "componentID": "my-redis",
                "componentMetadata": {
                    "Password": ""
                }
            },
            {
                "componentType": "mongo",
                "exposeComponent": true,
                "componentID": "my-mongo",
                "componentMetadata": {
                    "Password": ""
                }
            }
        ]
    }
}
```

Notice the Url of the code editor; you can use it to access the session using a
GUI similar to VsCode directly from your browser, without installing any
additional tools.


# License
AutoDev is MIT licensed.

