package main

import (
	"embed"
	"log"

	"github.com/spf13/viper"

	"go-mp4-server/pkg/videoserver"
)

//go:embed views
var viewsAsssets embed.FS

func main() {

	// Create video server
	cfg := videoserver.VideoServerCfg{
		Debug: true,
		Reload: true,
		EnableBaseAuth: true,
		ViewsAssets: viewsAsssets,
		BaseAuthConfigPath: viper.GetString("SERVER.AUTH.CONFIG.PATH"),
		EnvConfig: viper.GetViper(),
	}
	videoServer := videoserver.NewVideoServer(&cfg)

	// Start video server
	err := videoServer.App.Listen(":" + videoServer.GetConfig("SERVER.PORT"))
	if err != nil {
		log.Fatalf("Error starting server: %s", err)
	}
}
