package windows

import (
	"math"
	"strings"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const labelPictureIdleBudget = 8 * 1024 * 1024

type labelPictureKey struct {
	value                                                 string
	color                                                 platform.Color
	size, weight, scale                                   float32
	width, height, skip, baseline                         int
	fontGeneration, nativeGeneration, renderingGeneration uint32
	italic, allowLCD                                      bool
}

type labelPicture struct {
	key                 labelPictureKey
	cache               *labelPictureCache
	texture, view       uintptr
	x, y, width, height int
	lcd                 bool
	refs                int
	bytes               int
	previous, next      *labelPicture
}

type labelPictureCache struct {
	entries        map[labelPictureKey]*labelPicture
	oldest, newest *labelPicture
	idleBytes      int
	bytes          int
}

func (cache *labelPictureCache) unlink(picture *labelPicture) {
	if picture.previous != nil {
		picture.previous.next = picture.next
	} else {
		cache.oldest = picture.next
	}
	if picture.next != nil {
		picture.next.previous = picture.previous
	} else {
		cache.newest = picture.previous
	}
	picture.previous, picture.next = nil, nil
	cache.idleBytes -= picture.bytes
}

func (cache *labelPictureCache) acquire(key labelPictureKey) *labelPicture {
	picture := cache.entries[key]
	if picture == nil {
		return nil
	}
	if picture.refs == 0 {
		cache.unlink(picture)
	}
	picture.refs++
	return picture
}

func (picture *labelPicture) release() {
	picture.refs--
	if picture.refs != 0 {
		return
	}
	cache := picture.cache
	picture.previous = cache.newest
	if cache.newest != nil {
		cache.newest.next = picture
	} else {
		cache.oldest = picture
	}
	cache.newest = picture
	cache.idleBytes += picture.bytes
	cache.evict(labelPictureIdleBudget)
}

func (cache *labelPictureCache) evict(budget int) {
	for cache.oldest != nil && cache.idleBytes > budget {
		picture := cache.oldest
		cache.unlink(picture)
		delete(cache.entries, picture.key)
		cache.bytes -= picture.bytes
		release(&picture.view)
		release(&picture.texture)
	}
	if len(cache.entries) == 0 {
		cache.entries = nil
	}
}

func (cache *labelPictureCache) store(picture *labelPicture) {
	if cache.entries == nil {
		cache.entries = make(map[labelPictureKey]*labelPicture)
	}
	picture.key.value = strings.Clone(picture.key.value)
	picture.cache, picture.refs = cache, 1
	picture.bytes = picture.width*picture.height*4 + len(picture.key.value) + int(unsafe.Sizeof(*picture)) + int(unsafe.Sizeof(picture.key))
	cache.entries[picture.key] = picture
	cache.bytes += picture.bytes
}

func (renderer *renderer) pictureKey(label *labelTexture, scale float32, skip, width, height int) (labelPictureKey, bool) {
	value := label.label
	if renderer.noLabelPictures || value.Value == "" || value.Mono || len(value.Runs) != 0 || len(value.Bitmap) != 0 {
		return labelPictureKey{}, false
	}
	key := labelPictureKey{
		value: value.Value, color: value.Color,
		size: value.FontSize * scale, weight: value.Weight, italic: value.Italic, scale: scale,
		width: width, height: height, skip: skip, baseline: platform.TextBaseline(value, height, scale),
		fontGeneration: value.FontGeneration, nativeGeneration: sharedEngine.FontGeneration(),
		renderingGeneration: renderer.textGeneration,
		allowLCD:            (renderer.window != 0 && !renderer.usesComposition()) || renderer.color.Alpha >= 1,
	}
	return key, key == key
}

func (renderer *renderer) pictureChanged(label *labelTexture, scale float32, skip, width int) bool {
	if label.picture == nil {
		return false
	}
	height := int(math.Ceil(float64(label.label.Height * scale)))
	key, eligible := renderer.pictureKey(label, scale, skip, width, height)
	return !eligible || label.picture.key != key
}

func (renderer *renderer) invalidateText() {
	renderer.textGeneration++
	for index := range renderer.labels {
		renderer.labels[index].stale = true
	}
	renderer.labelPictures.evict(0)
	if renderer.direct != nil {
		renderer.direct.close()
		renderer.direct = nil
	}
}

func (label *labelTexture) usePicture(picture *labelPicture) {
	label.dropTexture()
	label.picture = picture
	label.texture, label.view = picture.texture, picture.view
	label.scale, label.skip = picture.key.scale, picture.key.skip
	label.width, label.height = picture.key.width, picture.key.height
	label.lcd, label.stale = picture.lcd, false
	label.spans = label.spans[:0]
}

func (renderer *renderer) reuseLabelPicture(label *labelTexture, scale float32) bool {
	skip, width, shown := label.window(scale)
	if !shown {
		return false
	}
	height := int(math.Ceil(float64(label.label.Height * scale)))
	key, eligible := renderer.pictureKey(label, scale, skip, width, height)
	if !eligible {
		return false
	}
	if picture := renderer.labelPictures.acquire(key); picture != nil {
		label.usePicture(picture)
		return true
	}
	return false
}

func picturePixelBounds(pixels []byte, width, height int) (left, top, right, bottom int) {
	left, top = width, height
	coverage := unsafe.Slice((*uint32)(unsafe.Pointer(&pixels[0])), width*height)
	for row := 0; row < height; row++ {
		line := coverage[row*width : (row+1)*width]
		first := 0
		for first < width && line[first] == 0 {
			first++
		}
		if first == width {
			continue
		}
		last := width - 1
		for last > first && line[last] == 0 {
			last--
		}
		left, top = min(left, first), min(top, row)
		right, bottom = max(right, last+1), max(bottom, row+1)
	}
	if right == 0 {
		return 0, 0, 1, 1
	}
	return
}

func (renderer *renderer) uploadLabelPicture(label *labelTexture, key labelPictureKey, pixels []byte, width, height int) error {
	if picture := renderer.labelPictures.acquire(key); picture != nil {
		label.usePicture(picture)
		return nil
	}
	left, top, right, bottom := picturePixelBounds(pixels, width, height)
	picture := &labelPicture{key: key, x: left, y: top, width: right - left, height: bottom - top, lcd: label.lcd}
	description := textureDesc{Width: uint32(picture.width), Height: uint32(picture.height), Mips: 1, Array: 1, Format: 87, Samples: 1, Usage: 1, Bind: 8}
	data := resourceData{Data: unsafe.Pointer(&pixels[(top*width+left)*4]), Pitch: uint32(width * 4)}
	if err := hresult(comCall(renderer.device, 5, uintptr(unsafe.Pointer(&description)), uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&picture.texture)))); err != nil {
		return err
	}
	if err := hresult(comCall(renderer.device, 7, picture.texture, 0, uintptr(unsafe.Pointer(&picture.view)))); err != nil {
		release(&picture.texture)
		return err
	}
	renderer.labelPictures.store(picture)
	label.usePicture(picture)
	renderer.labelRebuilds++
	return nil
}
