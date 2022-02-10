package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func setDefaults() {
	// level mode:
	// development
	// normal
	// production
	// viper.SetDefault("LOG.LEVEL", "development")
	viper.SetDefault("LOG.LEVEL", "normal")
	viper.SetDefault("SERVER.PORT", "3000")
	viper.SetDefault("VIDEO.DIR", "") // GOMP4_VIDEO_DIR="/media/pi/ADATA HM900/my_record/"
}

func init() {
	setDefaults()
	viper.AutomaticEnv()        // read in environment variables that match
	viper.SetEnvPrefix("gomp4") // will be uppercased automatically
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	fmt.Println(viper.AllSettings())
	// fmt.Println("xxx", viper.GetString("LOG.LEVEL"))
}
