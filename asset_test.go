package asset3d

import (
	"testing"
)

func TestGlb(t *testing.T) {
	g := GltfToMst{}
	g.Convert("./test/Xbot.glb")
}

func TestObj(t *testing.T) {
	g := GltfToMst{}
	g.Convert("/home/hj/workspace/flywave-3dtile-plugin/server/data/glb/0.glb")

}
