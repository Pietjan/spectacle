//go:build linux

package spectacle

import (
	"image"

	xdraw "golang.org/x/image/draw"
)

// scaleRGBA resamples img to n×n non-premultiplied RGBA.
func scaleRGBA(img image.Image, n int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, n, n))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return dst
}

// sniPixmap is one entry of the StatusNotifierItem IconPixmap a(iiay):
// width, height, ARGB32 in network byte order.
type sniPixmap struct {
	Width, Height int32
	Bytes         []byte
}

func sniPixmaps(img image.Image, sizes ...int) []sniPixmap {
	out := make([]sniPixmap, 0, len(sizes))
	for _, n := range sizes {
		rgba := scaleRGBA(img, n)
		argb := make([]byte, n*n*4)
		for i := 0; i < n*n; i++ {
			argb[i*4+0] = rgba.Pix[i*4+3] // A
			argb[i*4+1] = rgba.Pix[i*4+0] // R
			argb[i*4+2] = rgba.Pix[i*4+1] // G
			argb[i*4+3] = rgba.Pix[i*4+2] // B
		}
		out = append(out, sniPixmap{Width: int32(n), Height: int32(n), Bytes: argb})
	}
	return out
}

// notificationImage is the org.freedesktop.Notifications image-data
// hint: width, height, rowstride, hasAlpha, bitsPerSample, channels,
// RGBA bytes.
func notificationImage(img image.Image, n int) (int32, int32, int32, bool, int32, int32, []byte) {
	rgba := scaleRGBA(img, n)
	return int32(n), int32(n), int32(n * 4), true, 8, 4, rgba.Pix
}
