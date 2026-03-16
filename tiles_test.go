package asset3d

import (
	"testing"
)

func TestTilesObjToMst_Convert(t *testing.T) {
	dataDir := "./data/1/0131/Model/OBJ"

	origin, err := ReadTileOrigin(dataDir)
	if err != nil {
		t.Fatalf("ReadTileOrigin failed: %v", err)
	}
	t.Logf("Origin: %v", origin)

	converter := &TilesObjToMst{ApplyOrigin: false}
	mesh, bbox, err := converter.Convert(dataDir)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	t.Logf("Without origin - BBox: %v", bbox)
	t.Logf("Nodes: %d, Materials: %d", len(mesh.Nodes), len(mesh.Materials))

	converter2 := &TilesObjToMst{ApplyOrigin: true}
	mesh2, bbox2, err := converter2.Convert(dataDir)
	if err != nil {
		t.Fatalf("Convert with origin failed: %v", err)
	}
	t.Logf("With origin - BBox: %v", bbox2)
	t.Logf("Nodes: %d, Materials: %d", len(mesh2.Nodes), len(mesh2.Materials))
}
