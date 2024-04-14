package main

import (
	"embed"
	"log"

	"go-mp4-server/pkg/videoserver"
)

//go:embed views
var viewsAsssets embed.FS

func main() {

	// Create video server
	videoServer := videoserver.NewVideoServer(viewsAsssets)

	// Start video server
	err := videoServer.App.Listen(":" + videoServer.Config.GetString("SERVER.PORT"))
	if err != nil {
		log.Fatalf("Error starting server: %s", err)
	}
}
