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

// configViperMutex 保护全局配置加载时的 viper 全局状态操作
var configViperMutex sync.Mutex

type Config struct {
	Daemonize bool              `yaml:"daemonize" mapstructure:"daemonize"`
	PidFile   string            `yaml:"pidfile" mapstructure:"pidfile"`
	Socket    string            `yaml:"socket" mapstructure:"socket"`
	Log       Log               `yaml:"log" mapstructure:"log"`
	Env       map[string]string `yaml:"env,omitempty" mapstructure:"env,omitempty"`
}

type Log struct {
	Level        string `yaml:"level,omitempty" mapstructure:"level,omitempty"`
	FileEnabled  bool   `yaml:"file_enabled" mapstructure:"file_enabled"`
	FilePath     string `yaml:"file_path,omitempty" mapstructure:"file_path,omitempty"`
	FileSize     int    `yaml:"file_size,omitempty" mapstructure:"file_size,omitempty"`
	FileCompress bool   `yaml:"file_compress,omitempty" mapstructure:"file_compress,omitempty"`
	MaxAge       int    `yaml:"max_age,omitempty" mapstructure:"max_age,omitempty"`
	MaxBackups   int    `yaml:"max_backups,omitempty" mapstructure:"max_backups,omitempty"`
}

func setDefault() {
	viper.SetDefault("daemonize", true)
	viper.SetDefault("pidfile", constants.DaemonPidFilePath)
	viper.SetDefault("socket", constants.DaemonSockFilePath)
	viper.SetDefault("log", map[string]any{
		"Level":        constants.DefaultLogLevel,
		"FilePath":     constants.DaemonLogFilePath,
		"FileEnabled":  true,
		"FileCompress": false,
		"FileSize":     10,
		"MaxAge":       7,
		"MaxBackups":   7,
	})
}

func GetConfig() *Config {
	return config
}

func SetConfig(configFile string) {
	configViperMutex.Lock()
	defer configViperMutex.Unlock()

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
