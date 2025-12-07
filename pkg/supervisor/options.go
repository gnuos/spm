package supervisor

import (
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"strings"
	"sync"

	"spm/pkg/config"

	"github.com/spf13/viper"
)

// procfileViperMutex 保护 Procfile 配置加载时的 viper 全局状态操作
var procfileViperMutex sync.Mutex

type ProcfileOption struct {
	AppName   string
	WorkDir   string
	Procfile  string
	Env       map[string]string
	Processes map[string]*ProcessOption
}

type ProcessOption struct {
	Root       string
	PidRoot    string
	LogRoot    string
	StopSignal string
	NumProcs   int
	Env        map[string]string

	cmd []string
}

func LoadProcfileOption(cwd string, procfile string) (*ProcfileOption, error) {
	procfileViperMutex.Lock()
	defer procfileViperMutex.Unlock()
	var procOpts *ProcfileOption
	var optsFile = fmt.Sprintf("%s/%s", cwd, "Procfile.options")

	_, err := os.Stat(optsFile)
	if errors.Is(err, os.ErrNotExist) {
		viper.SetConfigName("Procfile.options")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("etc")
		viper.AddConfigPath("../etc")
	} else if err != nil {
		log.Fatal(err)
	} else {
		viper.SetConfigFile(optsFile)
	}

	appName, err := GetAppName(cwd)
	if err != nil {
		return nil, err
	}

	viper.SetDefault("appName", appName)
	viper.SetDefault("workDir", cwd)
	viper.SetDefault("procfile", procfile)
	viper.SetDefault("env", map[string]string{})

	err = viper.ReadInConfig()
	if err != nil && !errors.As(err, &viper.ConfigFileNotFoundError{}) {
		log.Fatalf("Error getting config file, %s", err)
		return nil, err
	}

	err = viper.Unmarshal(&procOpts)
	if err != nil {
		fmt.Println("Unable to decode into struct, ", err)
		return nil, err
	}

	procFileCfg, err := LoadProcfile(procOpts.Procfile)
	if err != nil {
		return nil, err
	}

	if !procFileCfg.IsValid() {
		return nil, errors.New(`invalid Procfile format, process name must be consist of 'a-z A-Z 0-9 - _'`)
	}

	if len(procOpts.Processes) > 0 {
		for k := range procOpts.Processes {
			if _, ok := (*procFileCfg)[k]; !ok {
				delete(procOpts.Processes, k)
			}
		}
	} else {
		procOpts.Processes = make(map[string]*ProcessOption)
	}

	if procOpts.WorkDir == "" {
		procOpts.WorkDir = cwd
	}

	for name, cmd := range *procFileCfg {
		opt, ok := procOpts.Processes[name]
		if !ok {
			opt = &ProcessOption{}
		}

		if opt.Root == "" {
			opt.Root = cwd
		}
		if opt.NumProcs <= 0 {
			opt.NumProcs = 1
		} else if opt.NumProcs > maxCpus {
			opt.NumProcs = maxCpus
		}

		if opt.PidRoot == "" {
			opt.PidRoot = config.GetRuntimeDir(cwd)
		}

		if opt.LogRoot == "" {
			opt.LogRoot = config.GetRuntimeDir(cwd)
		}

		if opt.StopSignal == "" {
			opt.StopSignal = "TERM"
		}

		if opt.Env == nil {
			opt.Env = maps.Clone(procOpts.Env)
		} else {
			opt.Env = Merge(procOpts.Env, opt.Env)
		}

		var args []string
		if strings.Contains(cmd, `"`) || strings.Contains(cmd, `'`) {
			args = []string{"sh", "-c", cmd}
		} else {
			args = strings.Split(cmd, " ")
		}

		opt.cmd = args
	}

	return procOpts, nil
}

func Merge(envs ...map[string]string) map[string]string {
	globalEnvs := maps.Clone(config.GetConfig().Env)
	for _, e := range envs {
		maps.Copy(globalEnvs, e)
	}

	return globalEnvs
}
