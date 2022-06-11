package asset3d

import (
	"fmt"
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

func TestGltf3(t *testing.T) {
	ph := "/home/hj/workspace/flywave-3dtile-plugin/tests/model/1_%d%s"
	ots := ObjToMst{}
	for i := 1; i < 10; i++ {
		mh, _, _ := ots.Convert(fmt.Sprintf(ph, i, ".obj"))
		doc, _ := mst.MstToGltf([]*mst.Mesh{mh})
		glftbts, _ := mst.GetGltfBinary(doc, 8)
		ph2 := fmt.Sprintf(ph, i, ".glb")
		f, _ := os.Create(ph2)
		f.Write(glftbts)
		f.Close()
	}
}
