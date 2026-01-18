package geo

import (
	"strings"
)

const (
	base32 = "0123456789bcdefghjkmnpqrstuvwxyz"
)

var (
	base32Map = map[byte]int{}
	neighbors = map[string]map[byte]string{
		"n": {'e': "p0r21436x8zb9dcf5h7kjnmqesgutwvy", 'o': "bc01fg45238967deuvhjyznpkmstqrwx"},
		"s": {'e': "14365h7k9dcfesgujnmqp0r2twvyx8zb", 'o': "238967debc01teleuvhjyznpkmstqrwx"},
		"e": {'e': "bc01fg45238967deuvhjyznpkmstqrwx", 'o': "p0r21436x8zb9dcf5h7kjnmqesgutwvy"},
		"w": {'e': "238967debc01fg45teleuvhjyznpkmstqrwx", 'o': "14365h7k9dcfesgujnmqp0r2twvyx8zb"},
	}
	borders = map[string]map[byte]string{
		"n": {'e': "prxz", 'o': "bcfguvyz"},
		"s": {'e': "028b", 'o': "0145hjnp"},
		"e": {'e': "bcfguvyz", 'o': "prxz"},
		"w": {'e': "0145hjnp", 'o': "028b"},
	}
)

func init() {
	for i := 0; i < len(base32); i++ {
		base32Map[base32[i]] = i
	}
}

// Encode converts latitude and longitude to a geohash string with given precision
func Encode(lat, lon float64, precision int) string {
	if precision <= 0 {
		precision = 6
	}
	if precision > 12 {
		precision = 12
	}

	minLat, maxLat := -90.0, 90.0
	minLon, maxLon := -180.0, 180.0

	var hash strings.Builder
	isEven := true
	bit := 0
	ch := 0

	for hash.Len() < precision {
		if isEven {
			mid := (minLon + maxLon) / 2
			if lon >= mid {
				ch |= 1 << (4 - bit)
				minLon = mid
			} else {
				maxLon = mid
			}
		} else {
			mid := (minLat + maxLat) / 2
			if lat >= mid {
				ch |= 1 << (4 - bit)
				minLat = mid
			} else {
				maxLat = mid
			}
		}
		isEven = !isEven
		bit++
		if bit == 5 {
			hash.WriteByte(base32[ch])
			bit = 0
			ch = 0
		}
	}

	return hash.String()
}

// Decode converts a geohash string to latitude and longitude
func Decode(hash string) (lat, lon float64) {
	minLat, maxLat := -90.0, 90.0
	minLon, maxLon := -180.0, 180.0
	isEven := true

	for i := 0; i < len(hash); i++ {
		c := hash[i]
		cd, ok := base32Map[c]
		if !ok {
			continue
		}
		for j := 4; j >= 0; j-- {
			bit := (cd >> j) & 1
			if isEven {
				mid := (minLon + maxLon) / 2
				if bit == 1 {
					minLon = mid
				} else {
					maxLon = mid
				}
			} else {
				mid := (minLat + maxLat) / 2
				if bit == 1 {
					minLat = mid
				} else {
					maxLat = mid
				}
			}
			isEven = !isEven
		}
	}

	lat = (minLat + maxLat) / 2
	lon = (minLon + maxLon) / 2
	return
}

// Neighbor returns the geohash of the neighbor in the specified direction
func Neighbor(hash string, direction string) string {
	if len(hash) == 0 {
		return ""
	}

	hash = strings.ToLower(hash)
	lastChar := hash[len(hash)-1]
	parent := hash[:len(hash)-1]

	var t byte = 'e'
	if len(hash)%2 == 0 {
		t = 'o'
	}

	if strings.ContainsRune(borders[direction][t], rune(lastChar)) && len(parent) > 0 {
		parent = Neighbor(parent, direction)
	}

	neighborChars := neighbors[direction][t]
	idx := strings.IndexByte(neighborChars, lastChar)
	if idx >= 0 {
		return parent + string(base32[idx])
	}

	return hash
}

// AllNeighbors returns all 8 neighboring geohashes plus the center
func AllNeighbors(hash string) []string {
	return []string{
		hash,
		Neighbor(hash, "n"),
		Neighbor(hash, "s"),
		Neighbor(hash, "e"),
		Neighbor(hash, "w"),
		Neighbor(Neighbor(hash, "n"), "e"),
		Neighbor(Neighbor(hash, "n"), "w"),
		Neighbor(Neighbor(hash, "s"), "e"),
		Neighbor(Neighbor(hash, "s"), "w"),
	}
}
