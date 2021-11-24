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
	g := ObjToMst{}
	mh, _, _ := g.Convert("/home/hj/workspace/GISCore/build/public/Resources/model/public/BGYbieshu2/BGYbieshu2.obj")
	f, _ := os.Create("/home/hj/workspace/GISCore/build/public/Resources/model/public/BGYbieshu2/BGYbieshu2.mst")
	for _, nd := range mh.Nodes {
		for i := range nd.Vertices {
			v := &nd.Vertices[i]
			y := v[1]
			z := -v[2]
			v[1] = z
			v[2] = y
		}
	}
	mst.MeshMarshal(f, mh)
}
