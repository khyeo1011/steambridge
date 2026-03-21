package utils

func SteamIDToTapCoords(steamID uint64) (a, b uint8) {
	z := (steamID ^ (steamID >> 30)) * 0xbf58476d1ce4e5b9

	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z = (z ^ (z >> 31))

	a = uint8(z & 0xFF)
	b = uint8((z >> 8) & 0xFF)
	return a, b
}
