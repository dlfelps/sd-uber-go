// Package geo implements geohash encoding/decoding and a spatial index for
// fast proximity queries on driver locations.
//
// Go Learning Note — What is a Geohash?
// A geohash is a way to encode a latitude/longitude pair into a short string.
// The key property is that nearby locations share a common prefix. For example,
// two points 100m apart might both start with "9q8yyk", while a point 10km away
// might start with "9q8yz". This lets you use string prefix matching for fast
// proximity searches instead of computing distances between all pairs.
//
// Precision determines the cell size:
//
//	1 → ~5000 km    4 → ~39 km     7 → ~153 m    10 → ~1.2 m
//	2 → ~1250 km    5 → ~5 km      8 → ~19 m     11 → ~15 cm
//	3 → ~156 km     6 → ~1.2 km    9 → ~2.4 m    12 → ~1.9 cm
//
// This project uses precision 6 (~1.2 km cells) — a good balance for
// ride-sharing where drivers within a few kilometers are relevant.
package geo

import (
	"strings"
)

// base32 is the geohash character set (32 characters). Note that 'a', 'i',
// 'l', and 'o' are excluded to avoid confusion with digits 0/1.
const (
	base32 = "0123456789bcdefghjkmnpqrstuvwxyz"
)

// Package-level lookup tables for geohash neighbor calculations.
// The 'e' key means "even length" and 'o' means "odd length" — the geohash
// algorithm alternates between longitude and latitude bits, so neighbors
// differ based on whether the hash length is even or odd.
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

// init() runs automatically when the package is first imported, before main().
//
// Go Learning Note — init() Functions:
// Every Go package can have one or more init() functions. They run once, in
// dependency order, when the program starts. Common uses: building lookup tables,
// registering plugins, and validating configuration. Avoid expensive or
// side-effect-heavy work in init() — it makes testing harder since init() always
// runs. Here we pre-compute a reverse lookup map from base32 characters to their
// index positions.
func init() {
	for i := 0; i < len(base32); i++ {
		base32Map[base32[i]] = i
	}
}

// Encode converts latitude and longitude to a geohash string with given precision.
//
// Algorithm overview (binary interleaving):
//  1. Start with the full range: lat [-90, 90], lon [-180, 180]
//  2. Alternate between longitude (even bits) and latitude (odd bits)
//  3. For each step, bisect the range and set bit=1 if value >= midpoint
//  4. Every 5 bits are encoded as one base32 character
//
// Go Learning Note — strings.Builder:
// strings.Builder is the idiomatic way to efficiently build strings in Go.
// It minimizes memory allocations by using an internal byte buffer. Before
// Go 1.10, the common pattern was bytes.Buffer. Never build strings with
// repeated concatenation (s += "x") in a loop — that creates a new string
// (and allocation) each iteration because Go strings are immutable.
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

// Decode converts a geohash string back to the center latitude and longitude
// of the encoded cell. This is the inverse of Encode — it recovers the
// bounding box by replaying the binary subdivision, then returns the center.
//
// Go Learning Note — Named Return Values:
// The signature `(lat, lon float64)` uses named return values. This serves as
// documentation (the caller knows which float64 is latitude vs longitude) and
// allows a bare `return` statement at the end. Named returns are idiomatic for
// short functions, but for longer functions, explicit returns are often clearer.
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

// Neighbor returns the geohash of the adjacent cell in the specified direction
// ("n", "s", "e", "w"). This is used to find the 8 surrounding cells for
// proximity searches. The algorithm works by looking at the last character of
// the hash and finding its neighbor using pre-computed lookup tables, recursing
// into the parent hash when the current character is on the border of its
// parent's cell.
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

// AllNeighbors returns all 8 neighboring geohashes plus the center (9 total).
// This creates a 3x3 grid of cells to search for nearby drivers. At precision 6,
// each cell is ~1.2 km, so the 3x3 grid covers roughly a 3.6 km x 3.6 km area.
// Diagonal neighbors (NE, NW, SE, SW) are computed by chaining two Neighbor calls.
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
