package relay

import (
	"net"
	"sync"
	"time"

	"github.com/go-gost/gosocks5"
	"github.com/go-gost/relay"
)

func StatusText(code uint8) string {
	switch code {
	case relay.StatusBadRequest:
		return "Bad Request"
	case relay.StatusForbidden:
		return "Forbidden"
	case relay.StatusHostUnreachable:
		return "Host Unreachable"
	case relay.StatusInternalServerError:
		return "Internal Server Error"
	case relay.StatusNetworkUnreachable:
		return "Network Unreachable"
	case relay.StatusServiceUnavailable:
		return "Service Unavailable"
	case relay.StatusTimeout:
		return "Timeout"
	case relay.StatusUnauthorized:
		return "Unauthorized"
	default:
		return ""
	}
}

type udpTunConn struct {
	net.Conn
	taddr net.Addr

	mu     sync.Mutex
	kaOnce sync.Once
	kaStop chan struct{}
}

func (c *udpTunConn) StartKeepalive(interval time.Duration) {
	if interval <= 0 {
		return
	}

	c.kaOnce.Do(func() {
		c.kaStop = make(chan struct{})
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			kaAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:1")
			for {
				select {
				case <-ticker.C:
					if kaAddr == nil {
						continue
					}
					c.mu.Lock()
					_, err := c.writeToLocked(nil, kaAddr)
					c.mu.Unlock()
					if err != nil {
						return
					}
				case <-c.kaStop:
					return
				}
			}
		}()
	})
}

func (c *udpTunConn) Close() error {
	if c.kaStop != nil {
		select {
		case <-c.kaStop:
		default:
			close(c.kaStop)
		}
	}
	return c.Conn.Close()
}

func UDPTunClientConn(c net.Conn, targetAddr net.Addr) net.Conn {
	return &udpTunConn{
		Conn:  c,
		taddr: targetAddr,
	}
}

func UDPTunClientPacketConn(c net.Conn) net.PacketConn {
	return &udpTunConn{
		Conn: c,
	}
}

func UDPTunServerConn(c net.Conn) net.PacketConn {
	return &udpTunConn{
		Conn: c,
	}
}

func (c *udpTunConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	socksAddr := gosocks5.Addr{}
	header := gosocks5.UDPHeader{
		Addr: &socksAddr,
	}
	dgram := gosocks5.UDPDatagram{
		Header: &header,
		Data:   b,
	}
	_, err = dgram.ReadFrom(c.Conn)
	if err != nil {
		return
	}

	n = len(dgram.Data)
	if n > len(b) {
		n = copy(b, dgram.Data)
	}
	addr, err = net.ResolveUDPAddr("udp", socksAddr.String())

	return
}

func (c *udpTunConn) Read(b []byte) (n int, err error) {
	n, _, err = c.ReadFrom(b)
	return
}

func (c *udpTunConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeToLocked(b, addr)
}

func (c *udpTunConn) writeToLocked(b []byte, addr net.Addr) (n int, err error) {
	socksAddr := gosocks5.Addr{}
	if err = socksAddr.ParseFrom(addr.String()); err != nil {
		return
	}

	header := gosocks5.UDPHeader{
		Addr: &socksAddr,
	}
	dgram := gosocks5.UDPDatagram{
		Header: &header,
		Data:   b,
	}
	dgram.Header.Rsv = uint16(len(dgram.Data))
	dgram.Header.Frag = 0xff // UDP tun relay flag, used by shadowsocks
	_, err = dgram.WriteTo(c.Conn)
	n = len(b)

	return
}

func (c *udpTunConn) Write(b []byte) (n int, err error) {
	return c.WriteTo(b, c.taddr)
}
