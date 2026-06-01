package asset3d

import (
	"testing"

	"github.com/flywave/go-mst"
)

func TestGLTF(t *testing.T) {
	convert := &GltfToMst{}

	mesh, _, err := convert.Convert("./famen01.glb")

	if err != nil {
		t.Error(err)
	}

	mst.MeshWriteTo("./famen01.mst", mesh)
}
