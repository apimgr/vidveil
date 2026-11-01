package branding

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRefreshGeneratesConfiguredVariants(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	input := image.NewRGBA(image.Rect(0, 0, 400, 200))
	input.Set(0, 0, color.RGBA{R: 255, A: 255})
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, input); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	cache := NewImageCache(root, ImageSources{Logo: source, Favicon: source, OGImage: source})
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh(): %v", err)
	}
	for _, name := range []string{
		"logo-original.png", "logo-200.png", "logo-50.png", "favicon-16.png",
		"favicon-32.png", "favicon-48.png", "favicon-180.png", "favicon-192.png",
		"favicon-512.png", "og-original.png", "og-1200x630.png",
	} {
		f, err := cache.Open(name)
		if err != nil {
			t.Errorf("Open(%q): %v", name, err)
			continue
		}
		decoded, _, decodeErr := image.Decode(f)
		_ = f.Close()
		if decodeErr != nil {
			t.Errorf("Decode(%q): %v", name, decodeErr)
			continue
		}
		var want image.Point
		switch name {
		case "logo-original.png", "og-original.png":
			want = image.Pt(400, 200)
		case "logo-200.png":
			want = image.Pt(200, 100)
		case "logo-50.png":
			want = image.Pt(50, 25)
		case "og-1200x630.png":
			want = image.Pt(1200, 630)
		default:
			var size int
			if _, err := fmt.Sscanf(name, "favicon-%d.png", &size); err != nil {
				t.Fatal(err)
			}
			want = image.Pt(size, size)
		}
		if got := decoded.Bounds().Size(); got != want {
			t.Errorf("%s dimensions = %v, want %v", name, got, want)
		}
	}
}

func TestOpenRejectsUnknownAsset(t *testing.T) {
	cache := NewImageCache(t.TempDir(), ImageSources{})
	if _, err := cache.Open("../../server.yml"); !os.IsNotExist(err) {
		t.Fatalf("Open traversal asset error = %v", err)
	}
}

func TestRefreshKeepsPreviousCacheOnInvalidReplacement(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.png")
	input := image.NewRGBA(image.Rect(0, 0, 60, 30))
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, input); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	cache := NewImageCache(root, ImageSources{Logo: source})
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("not an image"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cache.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh() expected decode failure")
	}
	if !cache.Has("logo-200.png") {
		t.Fatal("failed refresh removed the previous cached logo")
	}
}
