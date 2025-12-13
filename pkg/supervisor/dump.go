package supervisor

import (
	"spm/pkg/codec"
	"spm/pkg/config"

	"github.com/dgraph-io/badger/v4"
)

func (se *SpmSession) doDump() (*codec.ResponseMsg, codec.ResponseCtl) {
	opt := badger.DefaultOptions(config.GetConfig().DumpFile).
		WithValueThreshold(32).
		WithValueLogFileSize(64 << 20).
		WithNumCompactors(4).
		WithNumMemtables(4)

	db, err := badger.Open(opt)
	if err != nil {
		return se.errorResponse(err)
	}

	encoder, err := codec.GetEncoder()
	if err != nil {
		return se.errorResponse(err)
	}

	defer func() {
		_ = db.Close()
	}()

	for name, proj := range se.sv.projectTable.Iter() {
		txn := db.NewTransaction(true)

		metadata, err := encoder.Marshal(struct {
			WorkDir  string
			Procfile string
		}{
			WorkDir:  proj.WorkDir,
			Procfile: proj.Procfile,
		})
		if err != nil {
			se.logger.Error(err)
			_ = txn.Commit()
			continue
		}

		err = txn.SetEntry(badger.NewEntry([]byte(name), metadata))
		if err != nil {
			se.logger.Error(err)
			_ = txn.Commit()
			continue
		}

		for proc := range proj.procTable.Values() {
			opt := &ProcessOption{}
			opt.Root = proc.Options.Root
			opt.PidRoot = proc.Options.PidRoot
			opt.LogRoot = proc.Options.LogRoot
			opt.StopSignal = proc.Options.StopSignal
			opt.NumProcs = proc.Options.NumProcs
			opt.Env = make([]string, len(proc.Options.Env))
			_ = copy(opt.Env, proc.Options.Env)
			opt.Cmd = make([]string, len(proc.Options.Cmd))
			_ = copy(opt.Cmd, proc.Options.Cmd)
			opt.Order = proc.Options.Order

			data, err := encoder.Marshal(opt)
			if err != nil {
				se.logger.Error(err)
				opt = nil
				data = nil
				continue
			}

			err = txn.SetEntry(badger.NewEntry([]byte(proc.FullName), data))
			if err != nil {
				se.logger.Error(err)
			}
		}
		_ = txn.Commit()
	}

	return &codec.ResponseMsg{
		Code:    200,
		Message: "Save project list Successfully",
	}, codec.ResponseNormal
}
