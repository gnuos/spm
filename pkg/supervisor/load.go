package supervisor

import (
	"spm/pkg/codec"
	"spm/pkg/config"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/gnuos/fudge"
	"github.com/k0kubun/pp/v3"
)

func (se *SpmSession) doLoad() (*codec.ResponseMsg, codec.ResponseCtl) {
	dumpDB := config.GetConfig().DumpFile

	db, err := fudge.Open(dumpDB, fudge.DefaultConfig)
	if err != nil {
		return se.errorResponse(err)
	}

	defer func() {
		_ = db.Close()
	}()

	procOpts := make(map[string]*ProcfileOption, 0)

	keys, err := db.Keys(nil, 0, 0, true)
	if err != nil {
		return se.errorResponse(err)
	}

	for _, key := range keys {
		name := string(key)
		metadata := struct {
			WorkDir  string
			Procfile string
		}{}

		if !strings.Contains(name, "::") {
			opt := &ProcfileOption{}
			opt.AppName = name
			var val []byte
			if err := db.Get(name, &val); err != nil {
				se.logger.Error(err)
				opt = nil
				continue
			}

			if err := cbor.Unmarshal(val, &metadata); err != nil {
				se.logger.Error(err)
				opt = nil
			} else {
				opt.WorkDir = metadata.WorkDir
				opt.Procfile = metadata.Procfile
				opt.Env = make([]string, 0)
				opt.Processes = make(map[string]*ProcessOption)

				procOpts[name] = opt
			}
		} else {
			namePair := strings.Split(name, "::")
			appName := namePair[0]
			procName := namePair[1]
			appOpt, present := procOpts[appName]
			if present {
				opt := new(ProcessOption)
				var val []byte
				if err := db.Get(name, &val); err != nil {
					se.logger.Error(err)
					opt = nil
					continue
				}

				if err := cbor.Unmarshal(val, &opt); err != nil {
					se.logger.Error(err)
					se.errorResponse(err)
				} else {
					appOpt.Processes[procName] = opt
				}
			}
		}
	}

	_, _ = pp.Println(procOpts)

	for _, opts := range procOpts {
		// 第一遍注册项目
		_, _ = se.sv.UpdateApp(true, opts)

		// 第二遍reload进程表
		_, _ = se.sv.UpdateApp(false, opts)
	}

	return &codec.ResponseMsg{
		Code:    200,
		Message: "Load project list Successfully",
	}, codec.ResponseNormal
}
