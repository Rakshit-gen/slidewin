package slidewin

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"slices"
	"sort"
)

// pointsPerServer matches ketama, the scheme Twemproxy uses: 40 MD5 digests per
// server, each cut into four 32-bit points on the ring.
const pointsPerServer = 160

// Ring is a consistent hash ring. Adding or removing a server only moves the
// keys that land next to its points, roughly 1/N of them.
type Ring struct {
	points []uint32
	owner  map[uint32]string
}

// NewRing places each named server on the ring.
func NewRing(servers ...string) *Ring {
	r := &Ring{owner: map[uint32]string{}}
	for _, s := range servers {
		r.add(s)
	}
	return r
}

func (r *Ring) add(server string) {
	for i := 0; i < pointsPerServer/4; i++ {
		d := md5.Sum([]byte(fmt.Sprintf("%s-%d", server, i)))
		for j := 0; j < 4; j++ {
			p := binary.LittleEndian.Uint32(d[j*4:])
			if _, taken := r.owner[p]; taken {
				continue
			}
			r.owner[p] = server
			r.points = append(r.points, p)
		}
	}
	slices.Sort(r.points)
}

func hashKey(key string) uint32 {
	d := md5.Sum([]byte(key))
	return binary.LittleEndian.Uint32(d[:4])
}

// Lookup returns the server that owns key.
func (r *Ring) Lookup(key string) string {
	if len(r.points) == 0 {
		return ""
	}
	h := hashKey(key)
	i := sort.Search(len(r.points), func(i int) bool { return r.points[i] >= h })
	if i == len(r.points) {
		i = 0
	}
	return r.owner[r.points[i]]
}
