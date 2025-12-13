package supervisor

import (
	"spm/pkg/codec"
	"spm/pkg/config"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/fxamacker/cbor/v2"
)

func (se *SpmSession) doLoad() (*codec.ResponseMsg, codec.ResponseCtl) {
	opt := badger.DefaultOptions(config.GetConfig().DumpFile).
		WithValueThreshold(32).
		WithValueLogFileSize(64 << 20).
		WithNumCompactors(4).
		WithNumMemtables(4)

	db, err := badger.Open(opt)
	if err != nil {
		return se.errorResponse(err)
	}

	defer func() {
		_ = db.Close()
	}()

	procOpts := make(map[string]*ProcfileOption, 0)

	err = db.View(func(txn *badger.Txn) error {
		iterOpts := badger.DefaultIteratorOptions
		iterOpts.PrefetchSize = 10

		it := txn.NewIterator(iterOpts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			name := string(item.Key())
			metadata := struct {
				WorkDir  string
				Procfile string
			}{}

			if !strings.Contains(name, "::") {
				opt := &ProcfileOption{}
				opt.AppName = name
				err := item.Value(func(val []byte) error {
					return cbor.Unmarshal(val, &metadata)
				})

				if err != nil {
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
					err := item.Value(func(val []byte) error {
						return cbor.Unmarshal(val, &opt)
					})
					if err != nil {
						se.logger.Error(err)
					} else {
						appOpt.Processes[procName] = opt
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		se.errorResponse(err)
	}

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
