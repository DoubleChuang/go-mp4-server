package main

import (
	"log"

	"go-mp4-server/pkg/videoserver"
)

func main() {
	// Create video server
	videoServer := videoserver.NewVideoServer()

	// Start video server
	err := videoServer.App.Listen(":" + videoServer.Config.GetString("SERVER.PORT"))
	if err != nil {
		log.Fatalf("Error starting server: %s", err)
	}
}
