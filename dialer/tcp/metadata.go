package tcp

import (
	"time"

	md "github.com/go-gost/core/metadata"
)

const (
	dialTimeout = "dialTimeout"
	mark        = "so_mark"
)

const (
	defaultDialTimeout = 5 * time.Second
)

type metadata struct {
	dialTimeout time.Duration
	mark        int
}

func (d *tcpDialer) parseMetadata(md md.Metadata) (err error) {
	d.md.dialTimeout = md.GetDuration(dialTimeout)
	d.md.mark = md.GetInt(mark)
	return
}
