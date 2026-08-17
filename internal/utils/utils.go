package utils

import "fmt"

func IntIPtoString(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", ip>>24, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
}
