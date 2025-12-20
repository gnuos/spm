package supervisor

import (
	"spm/pkg/codec"
	"spm/pkg/config"

	"github.com/gnuos/fudge"
)

func (se *SpmSession) doDump() (*codec.ResponseMsg, codec.ResponseCtl) {
	dumpDB := config.GetConfig().DumpFile

	encoder, err := codec.GetEncoder()
	if err != nil {
		return se.errorResponse(err)
	}

	for name, proj := range se.sv.projectTable.Iter() {
		metadata, err := encoder.Marshal(struct {
			WorkDir  string
			Procfile string
		}{
			WorkDir:  proj.WorkDir,
			Procfile: proj.Procfile,
		})
		if err != nil {
			return se.errorResponse(err)
		}

		err = fudge.Set(dumpDB, name, metadata)
		if err != nil {
			return se.errorResponse(err)
		}

		for proc := range proj.procTable.Values() {
			opt := &ProcessOption{}
			opt.Root = proc.opts.Root
			opt.PidRoot = proc.opts.PidRoot
			opt.LogRoot = proc.opts.LogRoot
			opt.StopSignal = proc.opts.StopSignal
			opt.NumProcs = proc.opts.NumProcs
			opt.Env = make([]string, len(proc.opts.Env))
			_ = copy(opt.Env, proc.opts.Env)
			opt.Cmd = make([]string, len(proc.opts.Cmd))
			_ = copy(opt.Cmd, proc.opts.Cmd)
			opt.Order = proc.opts.Order

			data, err := encoder.Marshal(opt)
			if err != nil {
				se.logger.Error(err)
				opt = nil
				data = nil
				_ = fudge.Delete(dumpDB, name)

				continue
			}

			err = fudge.Set(dumpDB, proc.FullName, data)
			if err != nil {
				se.logger.Error(err)
			}
		}
	}

	return &codec.ResponseMsg{
		Code:    200,
		Message: "Save project list Successfully",
	}, codec.ResponseNormal
}
