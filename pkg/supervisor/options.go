package supervisor

import (
	"errors"
	"fmt"
	"log"
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
	Env       []string
	Processes map[string]*ProcessOption `yaml:"processes,omitempty"`
}

type ProcessOption struct {
	Root       string   `yaml:"root,omitempty"`
	PidRoot    string   `yaml:"pidRoot,omitempty"`
	LogRoot    string   `yaml:"logRoot,omitempty"`
	StopSignal string   `yaml:"stopSignal,omitempty"`
	NumProcs   int      `yaml:"numProcs,omitempty"`
	Env        []string `yaml:"env,omitempty"`

	cmd   []string
	order int
}

func LoadProcfileOption(cwd string, procfile string) (*ProcfileOption, error) {
	procfileViperMutex.Lock()
	defer procfileViperMutex.Unlock()
	var procOpts *ProcfileOption
	var optsFile = fmt.Sprintf("%s/%s", cwd, "Procfile.options")

	viper.SetDefault("env", []string{})
	viper.SetDefault("workDir", cwd)
	viper.SetDefault("procfile", procfile)
	viper.SetDefault("processes", make(map[string]*ProcessOption))
	viper.SetConfigType("yaml")

	_, err := os.Stat(optsFile)
	if errors.Is(err, os.ErrNotExist) {
		viper.SetConfigName("Procfile.options")
		viper.AddConfigPath(".")
		viper.AddConfigPath("etc")
		viper.AddConfigPath("../etc")
	} else if err != nil {
		log.Fatal(err)
	} else {
		viper.SetConfigFile(optsFile)
	}

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

	// 加载Procfile文件内容成功之后再设置项目名
	appName, err := GetAppName(cwd)
	if err != nil {
		return nil, err
	}

	if procOpts.AppName == "" {
		procOpts.AppName = appName
	}

	if len(procOpts.Processes) > 0 {
		for k := range procOpts.Processes {
			if _, ok := procFileCfg.Get(k); !ok {
				delete(procOpts.Processes, k)
			}
		}
	}

	order := 0
	task := procFileCfg.Oldest()
	for task != nil {
		name, cmd := task.Key, task.Value
		opt, ok := procOpts.Processes[name]
		if !ok {
			opt = &ProcessOption{}
			procOpts.Processes[name] = opt
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

		parentEnv := append(config.GetConfig().Env, procOpts.Env...)
		if opt.Env == nil {
			_ = copy(opt.Env, procOpts.Env)
		}

		opt.Env = append(parentEnv, opt.Env...)

		var args []string
		if strings.Contains(cmd, `"`) || strings.Contains(cmd, `'`) {
			args = []string{"sh", "-c", cmd}
		} else {
			args = strings.Split(cmd, " ")
		}

		opt.cmd = append(opt.cmd, args...)
		opt.order = order

		task = task.Next()
		order++
	}

	return procOpts, nil
}
