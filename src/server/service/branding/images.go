// Package branding processes operator supplied branding images into cached
// variants served by the WebUI.
package branding

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	stddraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/apimgr/vidveil/src/common/urlutil"
)

const maxImagePixels = int64(40_000_000)

// ImageSources defines optional local paths or remote URLs for the branding
// images. Empty fields keep the embedded defaults in use.
type ImageSources struct {
	Logo    string
	Favicon string
	OGImage string
}

// ImageCache stores scaled branding images in the application data directory.
type ImageCache struct {
	mu      sync.Mutex
	dir     string
	sources ImageSources
}

// NewImageCache creates a cache rooted at dir/branding.
func NewImageCache(dataDir string, sources ImageSources) *ImageCache {
	return &ImageCache{dir: filepath.Join(dataDir, "branding"), sources: sources}
}

// Refresh fetches configured remote images and regenerates all cached variants.
// A failed refresh leaves previously generated files available for serving.
func (c *ImageCache) Refresh(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("create branding cache: %w", err)
	}
	var errs []error
	for _, item := range []struct{ name, source string }{
		{"logo", c.sources.Logo}, {"favicon", c.sources.Favicon}, {"og", c.sources.OGImage},
	} {
		if strings.TrimSpace(item.source) == "" {
			continue
		}
		data, err := loadSource(ctx, item.source)
		if err == nil {
			err = c.generate(item.name, data)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s image: %w", item.name, err))
		}
	}
	return errors.Join(errs...)
}

// SetSources applies the latest server configuration before a refresh.
func (c *ImageCache) SetSources(sources ImageSources) {
	c.mu.Lock()
	c.sources = sources
	c.mu.Unlock()
}

// SeedDefaultOG installs the embedded OpenGraph fallback before remote
// sources are refreshed.
func (c *ImageCache) SeedDefaultOG(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("create branding cache: %w", err)
	}
	return c.generate("og", data)
}

// Has reports whether name is one of the generated files and exists in cache.
func (c *ImageCache) Has(name string) bool {
	if !allowedAsset(name) {
		return false
	}
	info, err := os.Stat(filepath.Join(c.dir, name))
	return err == nil && info.Mode().IsRegular()
}

// Open returns a generated image only for the fixed allowlist of asset names.
func (c *ImageCache) Open(name string) (*os.File, error) {
	if !allowedAsset(name) {
		return nil, os.ErrNotExist
	}
	return os.Open(filepath.Join(c.dir, name))
}

func allowedAsset(name string) bool {
	switch name {
	case "logo-original.png", "logo-200.png", "logo-50.png", "og-original.png", "og-1200x630.png",
		"favicon-16.png", "favicon-32.png", "favicon-48.png", "favicon-180.png", "favicon-192.png", "favicon-512.png":
		return true
	default:
		return false
	}
}

func loadSource(ctx context.Context, source string) ([]byte, error) {
	u, err := url.Parse(source)
	if err == nil && (strings.EqualFold(u.Scheme, "https") || strings.EqualFold(u.Scheme, "http")) {
		data, _, fetchErr := urlutil.FetchRemoteImage(ctx, source, urlutil.DefaultFetchRemoteImageConfig())
		return data, fetchErr
	}
	f, err := os.Open(source)
	if err != nil {
		return nil, fmt.Errorf("open local image: %w", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, 10*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("read local image: %w", err)
	}
	if len(data) > 10*1024*1024 {
		return nil, fmt.Errorf("local image exceeds 10 MiB")
	}
	return data, nil
}

func (c *ImageCache) generate(kind string, data []byte) error {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("read image dimensions: %w", err)
	}
	w, h := int64(config.Width), int64(config.Height)
	if w <= 0 || h <= 0 || w > maxImagePixels/h {
		return fmt.Errorf("image dimensions are invalid or too large")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}
	var variants map[string]image.Image
	switch kind {
	case "logo":
		variants = map[string]image.Image{"logo-original.png": img, "logo-200.png": fitWidth(img, 200), "logo-50.png": fitWidth(img, 50)}
	case "favicon":
		variants = make(map[string]image.Image)
		for _, size := range []int{16, 32, 48, 180, 192, 512} {
			variants[fmt.Sprintf("favicon-%d.png", size)] = fitBox(img, size, size)
		}
	case "og":
		variants = map[string]image.Image{"og-original.png": img, "og-1200x630.png": fitBox(img, 1200, 630)}
	default:
		return fmt.Errorf("unknown branding image type")
	}
	for name, variant := range variants {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, variant); err != nil {
			return fmt.Errorf("encode %s: %w", name, err)
		}
		if err := writeAtomic(c.dir, name, encoded.Bytes()); err != nil {
			return fmt.Errorf("cache %s: %w", name, err)
		}
	}
	return nil
}

func fitWidth(src image.Image, width int) image.Image {
	b := src.Bounds()
	outWidth := min(width, b.Dx())
	outHeight := max(1, b.Dy()*outWidth/b.Dx())
	dst := image.NewRGBA(image.Rect(0, 0, outWidth, outHeight))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, stddraw.Over, nil)
	return dst
}

func fitBox(src image.Image, width, height int) image.Image {
	b := src.Bounds()
	scale := min(float64(width)/float64(b.Dx()), float64(height)/float64(b.Dy()))
	outWidth := max(1, int(float64(b.Dx())*scale))
	outHeight := max(1, int(float64(b.Dy())*scale))
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	resized := image.NewRGBA(image.Rect(0, 0, outWidth, outHeight))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), src, b, stddraw.Over, nil)
	offset := image.Pt((width-outWidth)/2, (height-outHeight)/2)
	stddraw.Draw(canvas, image.Rectangle{Min: offset, Max: offset.Add(resized.Bounds().Size())}, resized, image.Point{}, stddraw.Over)
	return canvas
}

func writeAtomic(dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, ".branding-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if err := f.Chmod(0644); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}
