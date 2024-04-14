package videoserver

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/django"
	"github.com/spf13/viper"

	_ "go-mp4-server/pkg/config"
)

// VideoServer struct is used to store video server related information and methods
type VideoServer struct {
	App    *fiber.App
	Config *viper.Viper
}

// NewVideoServer function is used to create a new instance of the video server
func NewVideoServer() *VideoServer {
	// Initialize Fiber application
	engine := django.New("./views", ".html")
	engine.Reload(true)
	engine.Debug(true)

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Create video server instance
	videoServer := &VideoServer{
		App:    app,
		Config: viper.GetViper(),
	}

	// Configure static file serving
	videoServer.configureStaticFiles()

	// Configure routes
	videoServer.configureRoutes()

	// Return video server instance
	return videoServer
}

// configureStaticFiles method is used to configure static file serving
func (vs *VideoServer) configureStaticFiles() {
	vs.App.Static("/static", "./static")
	vs.App.Static("/videos", vs.Config.GetString("VIDEO.DIR"), fiber.Static{
		ByteRange: true,
	})
}

// configureRoutes method is used to configure routes
func (vs *VideoServer) configureRoutes() {
	vs.App.Get("/", vs.handleRoot)
	vs.App.Get("/video/:idx", vs.handleVideo)
	vs.App.Use(vs.handleNotFound)
}

// handleRoot method is used to handle root path requests
func (vs *VideoServer) handleRoot(c *fiber.Ctx) error {
	videos, err := vs.getMp4Files(vs.Config.GetString("VIDEO.DIR"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if len(videos) <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "no such mp4 file in ", vs.Config.GetString("VIDEO.DIR"))
	}

	log.Println("video:", videos)

	return c.Render("index", fiber.Map{
		"Title":            "go-mp4-server",
		"videoTitle":       videos[0],
		"videoSrc":         videos[0],
		"Videos":           videos,
		"next_video_url":   1,
		"current_video_id": 0,
	})
}

// handleVideo method is used to handle video requests
func (vs *VideoServer) handleVideo(c *fiber.Ctx) error {
	videos, err := vs.getMp4Files(vs.Config.GetString("VIDEO.DIR"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	idx, err := strconv.Atoi(c.Params("idx"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if idx < 0 || idx > len(videos)-1 {
		return vs.handleNotFound(c)
	}

	log.Println("videos[", idx, "]:", videos[idx])

	renderMap := fiber.Map{
		"Title":            "go-mp4-server",
		"videoTitle":       videos[idx],
		"videoSrc":         videos[idx],
		"Videos":           videos,
		"current_video_id": idx,
	}

	if idx > 0 {
		renderMap["previous_video_url"] = strconv.Itoa(idx - 1)
	}

	if idx < len(videos)-1 {
		renderMap["next_video_url"] = strconv.Itoa(idx + 1)
	}

	return c.Render("index", renderMap)
}

// handleNotFound method is used to handle 404 errors
func (vs *VideoServer) handleNotFound(ctx *fiber.Ctx) error {
	// Send a 404 status code initially
	code := fiber.StatusNotFound
	err := ctx.Status(code).Render(
		fmt.Sprintf("%d", code), //404.html
		fiber.Map{})

	if err != nil {
		// In case the Render html fails
		return ctx.Status(code).SendString("Internal Server Error")
	}

	return nil
}

// getMp4Files method is used to get all MP4 files in the specified directory
func (vs *VideoServer) getMp4Files(dir string) ([]string, error) {
	mp4files := []string{}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return mp4files, err
	}

	for _, file := range files {
		fileName := file.Name()
		if file.IsDir() {
			subDir := filepath.Join(dir, fileName)
			mp4filesInSubDir, err := vs.getMp4Files(subDir)
			if err != nil {
				return mp4files, err
			}
			mp4files = append(mp4files, mp4filesInSubDir...)
		} else { // IsFile
			if !strings.HasPrefix(fileName, ".") &&
				strings.HasSuffix(fileName, ".mp4") {
				url := strings.Replace(dir, vs.Config.GetString("VIDEO.DIR"), "/videos", 1)
				mp4files = append(mp4files, filepath.Join(url, fileName))
			}
		}
	}
	return mp4files, nil
}
