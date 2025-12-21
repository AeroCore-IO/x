package dtls

import (
	"time"

	mdata "github.com/go-gost/core/metadata"
	mdutil "github.com/go-gost/x/metadata/util"
)

const (
	defaultBufferSize = 1200
)

type metadata struct {
	mtu            int
	bufferSize     int
	flightInterval time.Duration
	mark           int
}

func (d *dtlsDialer) parseMetadata(md mdata.Metadata) (err error) {
	d.md.mark = mdutil.GetInt(md, "so_mark", "mark")
	d.md.mtu = mdutil.GetInt(md, "dtls.mtu", "mtu")
	d.md.bufferSize = mdutil.GetInt(md, "dtls.bufferSize", "bufferSize")
	if d.md.bufferSize <= 0 {
		d.md.bufferSize = defaultBufferSize
	}
	d.md.flightInterval = mdutil.GetDuration(md, "dtls.flightInterval", "flightInterval")
	return
}
