package supervisor

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"

	"spm/pkg/codec"
	"spm/pkg/config"
	"spm/pkg/logger"

	"github.com/fxamacker/cbor/v2"
	"go.uber.org/zap"
)

type SpmClient struct {
	sock   *rpcSocket
	logger *zap.SugaredLogger
}

func ClientRun(msg *codec.ActionMsg) []*codec.ProcInfo {
	c := new(SpmClient)
	c.logger = logger.Logging("spm-cli")

	conn, err := net.Dial("unix", config.GetConfig().Socket)
	if err != nil {
		c.logger.Error(err)
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return nil
	}

	defer func() {
		_ = conn.Close()
	}()

	c.sock = &rpcSocket{
		conn: conn,
	}

	var data []byte

	encoder, err := codec.GetEncoder()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return nil
	}

	data, err = encoder.Marshal(msg)
	if err != nil {
		c.logger.Error(err)
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return nil
	}

	size := make([]byte, strconv.IntSize)
	binary.BigEndian.PutUint64(size, uint64(len(data)))

	err = c.sock.Send(size)
	if err != nil {
		c.logger.Error(err)
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return nil
	}

	err = c.sock.Send(data)
	if err != nil {
		c.logger.Error(err)
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return nil
	}

	var length uint64
	data, err = c.sock.Recv(strconv.IntSize)
	if err != nil {
		if err != io.EOF {
			c.logger.Error(err)
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return nil
		}
	}

	if data != nil {
		length = binary.BigEndian.Uint64(data)
	}

	data, err = c.sock.Recv(length)
	if err != nil {
		if err != io.EOF {
			c.logger.Error(err)
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return nil
		}
	}

	if len(data) == 0 {
		return nil
	}

	var res = new(codec.ResponseMsg)
	err = cbor.Unmarshal(data, res)
	if err != nil {
		c.logger.Error(err)
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "%d\t%s\n\n", res.Code, res.Message)

	if res.Processes != nil {
		return res.Processes
	}

	return nil
}
