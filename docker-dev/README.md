The Docker Dev image is a containerised version of the developer environment, which contains all
tools required to develop the Community Operator.

How to run:

Create a `.env` file in this directory. This will allow parameterization of the docker-compose.yml file.

E.g. 

```bash
PATH_TO_PROJECT_ROOT="/absolute/path/to/project/root"
```

Build the dev environment image and run detached
```bash
cd docker-dev
docker-compose up --detach
```


Exec into container
```
docker exec -it ${docker_container_id} bash
```


Optional:
 
```bash
_get_docker_container_id(){
    echo "$(dcl | fzf | awk '{ print $1 }')"
}

de(){
    docker_container_id="$(_get_docker_container_id)"
    docker exec -it ${docker_container_id} bash
}
```

Note: Once connected to the container, the only additional setup is installing sonar. If you have mounted your .ssh
directory, you should be able to run:

```bash
pip install git+ssh://git@github.com/10gen/sonar.git@0.0.10
```


The `/workspace` will map directly to your host checkout of the project.

The following tools are installed.

`operator-sdk`, `jq`, `go`, `python3` (w/project packages installed), `controller-gen`, `kubectl`, `kustomize`, `docker`.
