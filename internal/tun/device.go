package tun

const MAXMTU = 1180

type TunInterface interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
	SetIP(ip uint32) error
	Name() string
}
