package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"spm/pkg/utils/constants"

	"github.com/spf13/viper"
)

var config *Config
var m sync.Mutex

type Config struct {
	Daemonize bool
	PidFile   string
	Socket    string
	Log       Log
	Env       map[string]string
}

type Log struct {
	Level        string
	FileEnabled  bool
	FilePath     string
	FileSize     int
	FileCompress bool
	MaxAge       int
	MaxBackups   int
}

func setDefault() {
	viper.SetDefault("daemonize", true)
	viper.SetDefault("pidfile", constants.DaemonPidFilePath)
	viper.SetDefault("socket", constants.DaemonSockFilePath)
	viper.SetDefault("log", map[string]any{
		"level":        constants.DefaultLogLevel,
		"filePath":     constants.DaemonLogFilePath,
		"fileEnabled":  true,
		"fileCompress": false,
		"fileSize":     10,
		"maxAge":       7,
		"maxBackups":   7,
	})
}

func GetConfig() *Config {
	return config
}

func SetConfig(configFile string) {
	m.Lock()
	defer m.Unlock()

	_, err := os.Stat(configFile)
	if errors.Is(err, os.ErrNotExist) {
		cfgName := fmt.Sprintf("%s.yml", constants.DefaultDaemonName)

		viper.SetConfigName(cfgName)
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("etc")
		viper.AddConfigPath("../etc")
		viper.AddConfigPath(constants.SpmHome)
	} else if err != nil {
		log.Fatal(err)
	} else {
		viper.SetConfigFile(configFile)
	}

	viper.SetEnvPrefix("SPM")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefault()

	err = viper.ReadInConfig()
	if err != nil && !errors.As(err, &viper.ConfigFileNotFoundError{}) {
		log.Fatalf("Error getting config file, %v", err)
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		fmt.Println("Unable to decode into struct, ", err)
	}
}

func GetRuntimeDir(cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		log.Fatal(err)
	}

	info, err := os.Stat(abs)
	if err == nil {
		if !info.IsDir() {
			log.Fatal(os.PathError{Op: "dirname", Path: cwd, Err: os.ErrInvalid})
		}
	}

	tmp := fmt.Sprintf("%s/tmp", abs)
	info, err = os.Stat(tmp)
	if err == nil {
		if info.IsDir() {
			return tmp
		} else {
			log.Fatal(os.PathError{Op: "mkdir", Path: tmp, Err: os.ErrExist})
		}
	} else {
		if err := os.MkdirAll(tmp, 0755); err != nil {
			log.Fatalf("create directory %q error: %v", tmp, err)
		}
	}

	return tmp
}
