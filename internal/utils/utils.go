package utils

import "fmt"

func SteamIDToTapCoords(steamID uint64) (a, b uint8) {
	z := (steamID ^ (steamID >> 30)) * 0xbf58476d1ce4e5b9

	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z = (z ^ (z >> 31))

	a = uint8(z & 0xFF)
	b = uint8((z >> 8) & 0xFF)
	return a, b
}

func IntIPtoString(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", ip>>24, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
}
