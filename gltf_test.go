package asset3d

import (
	"testing"

	"github.com/flywave/go-mst"
)

func TestGLTF(t *testing.T) {
	convert := &GltfToMst{}

	mesh, _, err := convert.Convert("./test/topside.glb")

	if err != nil {
		t.Error(err)
	}

	mst.MeshWriteTo("./test/topside.mst", mesh)
}
