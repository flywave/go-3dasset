package asset3d

import (
	"os"
	"testing"

	"github.com/flywave/go-mst"
)

func TestGlb(t *testing.T) {
	g := GltfToMst{}
	g.Convert("./test/Xbot.glb")
}

func TestObj(t *testing.T) {
	g := GltfToMst{}
	mh, _, _ := g.Convert("/home/hj/snap/dukto/16/Horse.glb")
	f, _ := os.Create("/home/hj/snap/dukto/16/Horse.mst")

	mst.MeshMarshal(f, mh)
}
