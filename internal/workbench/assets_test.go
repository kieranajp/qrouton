package workbench

import "testing"

func TestAssetMedia(t *testing.T) {
	for name, want := range map[string]Media{
		"x.png": {"image/png", MediaImage}, "x.PNG": {"image/png", MediaImage},
		"x.jpg": {"image/jpeg", MediaImage}, "x.JPEG": {"image/jpeg", MediaImage}, "x.jpeg": {"image/jpeg", MediaImage},
		"x.gif": {"image/gif", MediaImage}, "x.webp": {"image/webp", MediaImage}, "x.AVIF": {"image/avif", MediaImage},
		"x.mp4": {"video/mp4", MediaVideo}, "x.M4V": {"video/mp4", MediaVideo},
		"x.mov": {"video/quicktime", MediaVideo}, "x.MOV": {"video/quicktime", MediaVideo},
		"x.webm": {"video/webm", MediaVideo}, "dir.v/x.WebM": {"video/webm", MediaVideo},
	} {
		if got, ok := AssetMedia(name); !ok || got != want {
			t.Errorf("%s = %+v, %v", name, got, ok)
		}
	}
	for _, name := range []string{"x.mkv", "x.avi", "x.ogv", "x.svg", "x.heic", "x", "x.mp4.txt"} {
		if _, ok := AssetMedia(name); ok {
			t.Errorf("accepted %s", name)
		}
	}
}
