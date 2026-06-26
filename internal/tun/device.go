package tun

const MAXMTU = 1180

type TunInterface interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	// Unblock wakes any goroutine blocked in Read so it returns promptly.
	// It does NOT free device resources; call Close() after all goroutines exit.
	Unblock() error
	Close() error
	SetIP(ip uint32) error
	Name() string
}
