package asset3d

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flywave/go-mst"
)

func TestGlb(t *testing.T) {
	g := GltfToMst{}
	g.Convert("./test/Xbot.glb")
}

func TestObj(t *testing.T) {
	g := GltfToMst{}
	mh, _, _ := g.Convert("test/untitled.glb")
	doc, _ := mst.MstToGltf([]*mst.Mesh{mh})
	glftbts, _ := mst.GetGltfBinary(doc, 8)
	ph2 := "test/test1.glb"
	f2, _ := os.Create(ph2)
	f2.Write(glftbts)
	f2.Close()
}

func TestObj2(t *testing.T) {
	f, _ := os.Open("/home/hj/workspace/flywave-mesh-editor/data/out_79_88_tower_0_copy.mst")
	mst.MeshUnMarshal(f)
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
	ph := "/home/hj/workspace/flywave-mesh/data/test/test.obj"
	ots := ObjToMst{}
	mh, _, _ := ots.Convert(ph)
	ph1 := "/home/hj/workspace/flywave-mesh/data/test/test.mst"
	f1, _ := os.Create(ph1)
	mst.MeshMarshal(f1, mh)

	doc, _ := mst.MstToGltf([]*mst.Mesh{mh})
	glftbts, _ := mst.GetGltfBinary(doc, 8)
	ph2 := "/home/hj/workspace/flywave-mesh/data/test/test.glb"
	f2, _ := os.Create(ph2)
	f2.Write(glftbts)
	f2.Close()
}

func TestFBX(t *testing.T) {
	ph := "/home/hj/snap/dukto/16/md/"
	filepath.WalkDir(ph, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".fbx" {
			return nil
		}

		ots := FbxToMst{}
		mh, _, _ := ots.Convert(path)

		doc, _ := mst.MstToGltf([]*mst.Mesh{mh})
		glftbts, _ := mst.GetGltfBinary(doc, 8)
		ph2 := strings.TrimSuffix(path, ".fbx") + ".glb"
		f2, _ := os.Create(ph2)
		f2.Write(glftbts)
		f2.Close()
		return nil
	})

}

func TestFBX2(t *testing.T) {
	ph := "/home/hj/snap/dukto/16/md/联通大楼.fbx"
	ots := FbxToMst{}
	mh, _, _ := ots.Convert(ph)

	doc, _ := mst.MstToGltf([]*mst.Mesh{mh})
	glftbts, _ := mst.GetGltfBinary(doc, 8)
	ph2 := "/home/hj/snap/dukto/16/md/联通大楼.glb"
	f2, _ := os.Create(ph2)
	f2.Write(glftbts)
	f2.Close()
}
