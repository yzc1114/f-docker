# F-Docker
Imitating docker.
## Usage

1. compile under linux with `./build.sh`
2. run `f-docker` with sudo privilege

``` shell
sudo ./f-docker run [--mem] [--swap] [--pids] [--cpus] <image> <command>
# sudo ./f-docker run alpine /bin/sh 
sudo ./f-docker images
sudo ./f-docker rmi <image-id>
sudo ./f-docker ps
```