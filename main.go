package main

import (
	"fmt"
	_ "go-mp4-server/pkg/config"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/django"
	"github.com/spf13/viper"
)

func getMp4Files(dir string) ([]string, error) {
	mp4files := []string{}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return mp4files, err
	}

	for _, file := range files {
		fileName := file.Name()
		if file.IsDir() {
			subDir := filepath.Join(dir, fileName)
			mp4filesInSubDir, err := getMp4Files(subDir)
			if err != nil {
				return mp4files, err
			}
			mp4files = append(mp4files, mp4filesInSubDir...)
		} else { // IsFile
			// check filename has `.mp4` suffix and filename not has
			if !strings.HasPrefix(fileName, ".") &&
				strings.HasSuffix(fileName, ".mp4") {
				// replace Prefix
				url := strings.Replace(dir, viper.GetString("VIDEO.DIR"), "/videos", 1)
				mp4files = append(mp4files, filepath.Join(url, fileName))
			}
		}

	}
	return mp4files, nil
}

func main() {
	// Create a new engine
	engine := django.New("./views", ".html")

	engine.Reload(true) // Optional. Default: false

	// Debug will print each template that is parsed, good for debugging
	engine.Debug(true) // Optional. Default: false

	app := fiber.New(
		fiber.Config{
			Views: engine,
		},
	)

	app.Static("/static", "./static")
	app.Static("/videos", viper.GetString("VIDEO.DIR"),
		// https://github.com/gofiber/fiber/issues/253
		fiber.Static{
			ByteRange: true,
		})

	app.Get("/", func(c *fiber.Ctx) error {
		videos, err := getMp4Files(viper.GetString("VIDEO.DIR"))
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		if len(videos) <= 0 {
			return fiber.NewError(fiber.StatusBadRequest, "no such mp4 file in ", viper.GetString("VIDEO.DIR"))
		}
		log.Println("video:", videos)
		// Render with and extends
		return c.Render("index", fiber.Map{
			"Title":          "go-mp4-server",
			"videoSrc":       videos[0],
			"Videos":         videos,
			"next_video_url": 1,
		})
	})

	app.Get("/video/:idx",
		func(c *fiber.Ctx) error {
			videos, err := getMp4Files(viper.GetString("VIDEO.DIR"))
			if err != nil {
				return fiber.NewError(fiber.StatusInternalServerError, err.Error())
			}
			idx, err := strconv.Atoi(c.Params("idx"))
			if err != nil {
				return fiber.NewError(fiber.StatusBadRequest, err.Error())
			}
			if idx < 0 || idx >= len(videos) {
				return fiber.NewError(fiber.StatusBadRequest, "idx is out of range")
			}

			log.Println("videos[", idx, "]:", videos[idx])
			// Render with and extends

			renderMap := fiber.Map{
				"Title":    "go-mp4-server",
				"videoSrc": videos[idx],
				"Videos":   videos,
			}

			fmt.Println("idx: ", idx)

			if idx > 0 {
				fmt.Println("previous_video_url idx: ", idx)
				renderMap["previous_video_url"] = strconv.Itoa(idx - 1)
			}

			if idx < len(videos)-1 {
				fmt.Println("next_video_url idx: ", idx)
				renderMap["next_video_url"] = strconv.Itoa(idx + 1)
			}

			return c.Render("index", renderMap)
		})

	app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(404) // => 404 "Not Found"
	})

	// https://stackoverflow.com/questions/38069584/golang-http-webserver-provide-video-mp4
	app.Listen(":" + viper.GetString("SERVER.PORT"))
}
