## Docker build

```
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
docker buildx create --name arm64_buildkit --platform linux/arm64
docker buildx inspect arm64_buildkit --bootstrap
docker buildx use arm64_buildkit

docker buildx build --push --platform linux/arm/v6 -f dockerfile -t doublehub/go-mp4-server .
```

If you encounter an issue with TLS timeouts when compile code on MacOS, click the following link to resolve it
- https://github.com/docker/buildx/issues/350
- https://stackoverflow.com/questions/61612158/docker-buildx-build-fails-with-tls-handshake-timeout-while-docker-pull-works/61620325#61620325

## Parameters

GOMP4_VIDEO_DIR: the mp4 server will get all mp4 file from the directory

## Docker Run
example:
```
docker run \
    -d \
    --name go-mp4-server \
    --restart always \
    -p 30000:3000 \
    -v $PWD/videos:/videos \
    -e GOMP4_VIDEO_DIR=/videos \
    doublehub/go-mp4-server
```
## Docker Compose

Please follow the [docker-compose.yml](docker-compose.yml) to set the volume and environment

Start service
```
docker compose up -d
```
Shutdown service
```
docker compose down
```

## How to develop


### Build code on local enviornment
follow the below command to compile the binary, after successfully compile the binary will place in `bin` folder
```
make local
```


Note: The `views` and `static` folder should be placed in the directory you execute binary
```
# Set the video directory, and the mp4s in this folder will become a playlist
export GOMP4_VIDEO_DIR=`pwd`/videos

# Set the port
export GOMP4_SERVER_PORT=30080

# Set the auth config
export GOMP4_SERVER_AUTH_CONFIG_PATH=./auth.json

# Execute the binary
./bin/go-mp4-server
```
## TODO:
- [x] videojs
- [x] github ci build
- [x] dockerfile
- [ ] print log
- [ ] list all video performance
- [ ] refactor 
- [x] makefile
- [x] html layout

## Ref:
- [fiber file streaming](https://github.com/gofiber/fiber/issues/253)
- [templete range](https://stackoverflow.com/questions/67079636/rendering-templates-in-a-go-fiber-application)
- [django templete](https://github.com/gofiber/template/tree/master/django)
- [django templete tag](https://github.com/flosch/pongo2#tags)