package project

import (
	"github.com/pterm/pterm"
	"github.com/spf13/viper"
	"github.com/varrcan/dl/helper"
	"os"
	"path/filepath"
	"strings"
)

//Env Project variables
var Env *viper.Viper

//LoadEnv Get variables from .env file
func LoadEnv() {
	Env = viper.New()

	Env.AddConfigPath("./")
	Env.SetConfigFile(".env")
	Env.SetConfigType("env")
	err := Env.ReadInConfig()
	if err != nil {
		pterm.FgRed.Printfln(".env file not found. Please run the command: dl env")
	}

	setDefaultEnv()
	setComposeFiles()
}

//setNetworkName Set network name from project name
func setDefaultEnv() {
	projectName := Env.GetString("APP_NAME")

	if len(projectName) == 0 {
		pterm.FgRed.Printfln("The APP_NAME variable is not defined! Please initialize .env file.")
		os.Exit(1)
	}

	res := strings.ReplaceAll(projectName, ".", "")
	Env.SetDefault("NETWORK_NAME", res)

	dir, _ := os.Getwd()
	Env.SetDefault("PWD", dir)

	Env.SetDefault("REDIS", false)
	Env.SetDefault("REDIS_PASSWORD", "pass")
	Env.SetDefault("MEMCACHED", false)
}

//setComposeFile Set docker-compose files
func setComposeFiles() {
	var files []string
	confDir, _ := helper.ConfigDir()
	phpVersion := Env.GetString("PHP_VERSION")

	if len(phpVersion) == 0 {
		pterm.FgRed.Printfln("The PHP_VERSION variable is not defined! Please initialize .env file.")
		os.Exit(1)
	}

	images := map[string]string{
		"mysql":     confDir + "/config-files/docker-compose-mysql.yaml",
		"fpm":       confDir + "/config-files/docker-compose-fpm.yaml",
		"apache":    confDir + "/config-files/docker-compose-apache.yaml",
		"redis":     confDir + "/config-files/docker-compose-redis.yaml",
		"memcached": confDir + "/config-files/docker-compose-memcached.yaml",
	}

	for imageType, imageComposeFile := range images {
		if strings.Contains(phpVersion, imageType) {
			files = append(files, imageComposeFile)
		}
	}

	if Env.GetFloat64("MYSQL_VERSION") > 0 {
		files = append(files, images["mysql"])
	}
	if Env.GetBool("REDIS") == true {
		files = append(files, images["redis"])
	}
	if Env.GetBool("MEMCACHED") == true {
		files = append(files, images["memcached"])
	}

	containers := strings.Join(files, ":")
	Env.SetDefault("COMPOSE_FILE", containers)
}

//CmdEnv Getting variables in the "key=value" format
func CmdEnv() []string {
	keys := Env.AllKeys()
	var env []string

	for _, key := range keys {
		value := Env.GetString(key)
		env = append(env, strings.ToUpper(key)+"="+value)
	}

	return env
}

//IsEnvFileExists checking for the existence of .env file
func IsEnvFileExists() bool {
	dir, _ := os.Getwd()
	env := filepath.Join(dir, ".env")

	_, err := os.Stat(env)

	if err != nil {
		return false
	}

	return true
}
