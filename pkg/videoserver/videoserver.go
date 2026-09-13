package videoserver

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/django/v3"
	"github.com/spf13/viper"
	"github.com/gofiber/fiber/v2/middleware/session"

	_ "go-mp4-server/pkg/config"
)

// VideoServer struct is used to store video server related information and methods
type VideoServer struct {
	App    *fiber.App
	Config *VideoServerCfg
	TotpStore *TotpStore
	SessionStore *session.Store
}

type VideoServerCfg struct {
	// Engine Config
	Debug bool
	Reload bool
	// Base Auth
	EnableBaseAuth bool
	BaseAuthConfigPath string
	EnvConfig *viper.Viper
	ViewsAssets embed.FS
	// TOTP Config
	TotpConfigPath string
	TotpIssuer string
}


// NewVideoServer function is used to create a new instance of the video server
func NewVideoServer(cfg *VideoServerCfg) *VideoServer {
	// Initialize Fiber application
	engine := django.NewPathForwardingFileSystem(
		http.FS(cfg.ViewsAssets),
		"/views",
		".django",
	)
	engine.Reload(cfg.Reload)
	engine.Debug(cfg.Debug)

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Create video server instance
	videoServer := &VideoServer{
		App:    app,
		Config: cfg,
	}

	videoServer.TotpStore = NewTotpStore(cfg.TotpConfigPath)

	// Configure auth (session + middleware) before any routes/statics
	videoServer.configureAuth()

	// Configure static file serving
	videoServer.configureStaticFiles()

	// Configure routes
	videoServer.configureRoutes()

	// Return video server instance
	return videoServer
}

func (vs *VideoServer) GetConfig(key string) string{
	return vs.Config.EnvConfig.GetString(key)
}

// configureStaticFiles method is used to configure static file serving
func (vs *VideoServer) configureStaticFiles() {
	vs.App.Static("/static", "./static")
	vs.App.Static("/videos", vs.GetConfig("VIDEO.DIR"), fiber.Static{
		ByteRange: true,
	})
}

// configureRoutes method is used to configure routes
func (vs *VideoServer) configureRoutes() {
	vs.App.Get("/login", vs.handleLoginPage)
	vs.App.Post("/login", vs.handleLogin)
	vs.App.Get("/verify", vs.handleVerifyPage)
	vs.App.Post("/verify", vs.handleVerify)
	vs.App.Get("/logout", vs.handleLogout)
	vs.App.Get("/settings", vs.handleSettings)
	vs.App.Post("/settings/2fa/enable", vs.handleEnable2FA)
	vs.App.Post("/settings/2fa/confirm", vs.handleConfirm2FA)
	vs.App.Post("/settings/2fa/cancel", vs.handleCancel2FA)
	vs.App.Post("/settings/2fa/disable", vs.handleDisable2FA)
	vs.App.Get("/", vs.handleVideo)
	vs.App.Get("/video/:idx", vs.handleVideo)
	vs.App.Use(vs.handleNotFound)
}

// handleVideo method is used to handle video requests
func (vs *VideoServer) handleVideo(c *fiber.Ctx) error {
	videos, err := vs.getMp4Files(vs.GetConfig("VIDEO.DIR"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	idx, err := strconv.Atoi(c.Params("idx", "0"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if idx < 0 || idx > len(videos)-1 {
		return vs.handleNotFound(c)
	}

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
		isVideoPath := strings.HasPrefix(c.Path(), "/video")
		if isVideoPath {
			renderMap["next_video_url"] = strconv.Itoa(idx + 1)
		} else {
			renderMap["next_video_url"] = "video/" + strconv.Itoa(idx+1)
		}
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
	files, err := os.ReadDir(dir)
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
				url := strings.Replace(dir, vs.GetConfig("VIDEO.DIR"), "/videos", 1)
				mp4files = append(mp4files, filepath.Join(url, fileName))
			}
		}
	}
	return mp4files, nil
}
