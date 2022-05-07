# F-Docker
Imitating docker.
## Usage

1. compile under linux with `./build.sh`
2. run `f-docker` with sudo privilege

``` shell
# docker run
sudo ./f-docker run [--mem] [--swap] [--pids] [--cpus] <image> <command>
# docker images
sudo ./f-docker images
# docker rmi <image-id>
sudo ./f-docker rmi <image-id>
# docker ps -a
sudo ./f-docker ps
```