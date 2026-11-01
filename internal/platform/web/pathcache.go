//go:build js && wasm

package web

import (
	"math"
	"syscall/js"

	"github.com/vivaui/viva/ui"
)

const (
	// pathUnit is the grid, in 1/pathUnit of a pixel, that a cached path's
	// coordinates snap to. 1/64 px is far below what a rasterizer can show, and
	// makes the same shape found at a different fractional position the same key.
	pathUnit = 64
	// maxCachedPaths bounds the cache; past it, paths not used by the last
	// two frames are dropped.
	maxCachedPaths = 4096
)

// pathCache keeps one Path2D for each distinct run of solid triangles (an icon,
// a glyph's strokes), so a frame that draws the same shapes again costs a fill
// per shape instead of several browser calls per triangle.
//
// A path holds the run's geometry relative to its first point and is drawn
// translated to where the run is, so a shape is found again wherever it moves:
// scrolling a list of icons reuses every path. The cache is owned by the
// runtime and touched only from the event loop.
type pathCache struct {
	paths  map[uint64]cachedPath
	frame  uint64
	built  int // paths built in the current frame
	reused int // paths found in the current frame
}

type cachedPath struct {
	path js.Value
	used uint64 // frame that last drew it
}

// beginFrame starts a frame's accounting.
func (cache *pathCache) beginFrame() {
	cache.frame++
	cache.built, cache.reused = 0, 0
}

// endFrame drops what has gone unused once the cache is over its bound.
func (cache *pathCache) endFrame() {
	if len(cache.paths) <= maxCachedPaths {
		return
	}
	for key, entry := range cache.paths {
		if entry.used+1 < cache.frame {
			delete(cache.paths, key)
		}
	}
}

// path returns the Path2D of run, building it on first sight. Its triangles
// are wound the same way so overlapping ones fill as one shape.
func (cache *pathCache) path(run []ui.Triangle) js.Value {
	base := run[0].Points[0]
	key := runKey(run, base)
	if entry, ok := cache.paths[key]; ok {
		entry.used = cache.frame
		cache.paths[key] = entry
		cache.reused++
		return entry.path
	}
	path := js.Global().Get("Path2D").New()
	for _, triangle := range run {
		p := triangle.Points
		if cross(p) < 0 {
			p[1], p[2] = p[2], p[1]
		}
		for index, point := range p {
			x, y := snap(point.X-base.X), snap(point.Y-base.Y)
			if index == 0 {
				path.Call("moveTo", x, y)
			} else {
				path.Call("lineTo", x, y)
			}
		}
		path.Call("closePath")
	}
	if cache.paths == nil {
		cache.paths = make(map[uint64]cachedPath)
	}
	cache.paths[key] = cachedPath{path: path, used: cache.frame}
	cache.built++
	return path
}

// snap rounds a coordinate to the cache grid.
func snap(value float32) float32 {
	return float32(math.Round(float64(value)*pathUnit)) / pathUnit
}

func cross(p [3]ui.Point) float32 {
	return (p[1].X-p[0].X)*(p[2].Y-p[0].Y) - (p[2].X-p[0].X)*(p[1].Y-p[0].Y)
}

// runKey hashes a run's snapped geometry relative to base (FNV-1a).
func runKey(run []ui.Triangle, base ui.Point) uint64 {
	const prime = 1099511628211
	hash := uint64(14695981038295752099)
	mix := func(word uint32) { hash = (hash ^ uint64(word)) * prime }
	mix(uint32(len(run)))
	for _, triangle := range run {
		for _, point := range triangle.Points {
			mix(uint32(int32(math.Round(float64(point.X-base.X) * pathUnit))))
			mix(uint32(int32(math.Round(float64(point.Y-base.Y) * pathUnit))))
		}
	}
	return hash
}
