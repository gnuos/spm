package codec

import (
	"github.com/fxamacker/cbor/v2"
)

var encodeMode cbor.EncMode

func GetEncoder() (cbor.EncMode, error) {
	opts := cbor.CoreDetEncOptions()
	opts.Time = cbor.TimeUnix
	var err error

	if encodeMode == nil {
		encodeMode, err = opts.EncMode()
	}

	return encodeMode, err
}
