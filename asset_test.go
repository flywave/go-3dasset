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
	mh, _, _ := g.Convert("/home/hj/snap/dukto/16/model_11_12.glb")
	f, _ := os.Create("/home/hj/snap/dukto/16/model_11_12.mst_exchange.mst")
	mst.MeshMarshal(f, mh)
}

func TestObjTomst(t *testing.T) {
	g := ObjToMst{}
	mh, _, _ := g.Convert("/home/hj/snap/dukto/16/model_insulator_50.obj")
	f, _ := os.Create("/home/hj/snap/dukto/16/model_insulator_50.obj.mst")
	mst.MeshMarshal(f, mh)
	f.Close()
}

func TestObjTomst2(t *testing.T) {
	f1, _ := os.Open("/tmp/2426425825/model_11_12.mst_exchange.mst")
	mh := mst.MeshUnMarshal(f1)
	doc, _ := mst.MstToGltf([]*mst.Mesh{mh})
	glftbts, _ := mst.GetGltfBinary(doc, 8)
	ph2 := "/tmp/2426425825/model_11_12.mst_exchange.mst.glb"
	f, _ := os.Create(ph2)
	f.Write(glftbts)
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

func TestGltf4(t *testing.T) {
	ph := "/home/hj/snap/dukto/16/model_hill_1.mst.obj"
	ots := ObjToMst{}
	mh, _, _ := ots.Convert(ph)
	doc, _ := mst.MstToGltf([]*mst.Mesh{mh})
	glftbts, _ := mst.GetGltfBinary(doc, 8)
	ph2 := "/home/hj/snap/dukto/16/model_hill_1.mst.glb"
	f, _ := os.Create(ph2)
	f.Write(glftbts)
	f.Close()
}
