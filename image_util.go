package asset3d

import (
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"

	"github.com/chai2010/tiff"
	mst "github.com/flywave/go-mst"
	"golang.org/x/image/bmp"
)

func convertTex(path string, texId int) (*mst.Texture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	_, ft, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	f.Seek(0, 0)
	img, err := readImage(f, ft)
	if err != nil {
		return nil, err
	}
	bd := img.Bounds()
	buf := []byte{}
	for y := 0; y < bd.Dy(); y++ {
		for x := 0; x < bd.Dx(); x++ {
			cl := img.At(x, y)
			r, g, b, a := color.RGBAModel.Convert(cl).RGBA()
			buf = append(buf, byte(r&0xff), byte(g&0xff), byte(b&0xff), byte(a&0xff))
		}
	}
	f.Close()

	t := &mst.Texture{}
	t.Id = int32(texId)
	t.Format = mst.TEXTURE_FORMAT_RGBA
	t.Size = [2]uint64{uint64(bd.Dx()), uint64(bd.Dy())}
	t.Compressed = mst.TEXTURE_COMPRESSED_ZLIB
	t.Data = mst.CompressImage(buf)
	return t, nil
}

func readImage(rd io.Reader, ft string) (image.Image, error) {
	switch ft {
	case "jpeg", "jpg":
		return jpeg.Decode(rd)
	case "png":
		return png.Decode(rd)
	case "gif":
		return gif.Decode(rd)
	case "bmp":
		return bmp.Decode(rd)
	case "tif", "tiff":
		return tiff.Decode(rd)
	default:
		return nil, errors.New("unknow format")
	}
}
