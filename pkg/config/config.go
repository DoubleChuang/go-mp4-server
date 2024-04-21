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
	viper.SetDefault("SERVER.AUTH.CONFIG.PATH", "./auth.json") // auth.json"
}

func init() {
	setDefaults()
	// viper.AddConfigPath(".")
	// viper.SetConfigFile(".env")
	viper.AutomaticEnv()        // read in environment variables that match
	viper.SetEnvPrefix("gomp4") // will be uppercased automatically

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	fmt.Println(viper.AllKeys())

	fmt.Println("video dir", viper.GetString("VIDEO.DIR"))
	fmt.Println("port", viper.GetString("SERVER.PORT"))
}
