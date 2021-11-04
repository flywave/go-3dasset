package asset3d

import "testing"

func TestGlb(t *testing.T) {
	g := GltfToMst{}
	g.Convert("./test/Horse.glb")
}
