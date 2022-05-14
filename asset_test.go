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
	g.Convert("/home/hj/workspace/flywave-3dtile-plugin/server/data/glb/0.glb")

}

func TestObjTomst(t *testing.T) {
	g := ObjToMst{}
	mh, _, _ := g.Convert("/home/hj/workspace/go-3dasset/test/female02/female02_vertex_colors.obj")
	f, _ := os.Create("/home/hj/workspace/go-3dasset/test/female02/female02_vertex_colors.obj.mst")
	mst.MeshMarshal(f, mh)
	f.Close()
}
