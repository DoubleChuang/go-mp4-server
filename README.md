## docker build


```
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
docker buildx create --name arm64_buildkit --platform linux/arm64
docker buildx inspect arm64_buildkit --bootstrap
docker buildx use arm64_buildkit

docker buildx build --push --platform linux/arm/v6 -f dockerfile -t doublehub/go-mp4-server .
```

## Run

```
docker run --rm -p 30000:3000 -v /media/pi/ADATA\ HM900/my_record/:/videos -e GOMP4_VIDEO_DIR=/videos doublehub/go-mp4-server
```

## run on pi3
```
# copy  the views directory to the directory that place the binary

export GOMP4_VIDEO_DIR=/media/pi/ADATA\ HM900/ZIP

export GOMP4_VIDEO_DIR=/media/pi/ADATA\ HM900/my_record/
export GOMP4_SERVER_PORT=30080
./go-mp4-server
```
## TODO:
- [ ] videojs
- [ ] github ci build
- [ ] dockerfile
- [ ] print log
- [ ] list all video performance
- [ ] refactor 
- [ ] makefiel
- [ ] html layout

## Ref:
- [fiber file streaming](https://github.com/gofiber/fiber/issues/253)
- [templete range](https://stackoverflow.com/questions/67079636/rendering-templates-in-a-go-fiber-application)
- [django templete](https://github.com/gofiber/template/tree/master/django)
- [django templete tag](https://github.com/flosch/pongo2#tags)