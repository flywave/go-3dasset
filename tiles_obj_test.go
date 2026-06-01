package asset3d

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	gobj "github.com/flywave/go-obj"
	"github.com/flywave/go3d/vec3"
)

// --- parseFloat ---

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"518078.000000", 518078},
		{"4080366.000000", 4080366},
		{"0.000000", 0},
		{"-500.5", -500.5},
		{"  100.5  ", 100.5},
		{"abc", 0},
	}
	for _, tt := range tests {
		got := parseFloat(tt.input)
		if math.Abs(got-tt.want) > 0.001 {
			t.Errorf("parseFloat(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

// --- calculateNormal ---

func TestCalculateNormal(t *testing.T) {
	c := &TilesObjToMst{}
	v0 := vec3.T{0, 0, 0}
	v1 := vec3.T{1, 0, 0}
	v2 := vec3.T{0, 1, 0}
	n := c.calculateNormal(v0, v1, v2)
	l := math.Sqrt(float64(n[0]*n[0] + n[1]*n[1] + n[2]*n[2]))
	if math.Abs(l-1.0) > 0.001 {
		t.Errorf("normal length = %f, want 1.0", l)
	}
	if math.Abs(float64(n[2])-1.0) > 0.001 {
		t.Errorf("normal.z = %f, want ~1.0 (facing +Z)", n[2])
	}
}

func TestCalculateNormal_Degenerate(t *testing.T) {
	c := &TilesObjToMst{}
	n := c.calculateNormal(vec3.T{0, 0, 0}, vec3.T{0, 0, 0}, vec3.T{0, 0, 0})
	if n != (vec3.T{0, 1, 0}) && n != (vec3.T{0, 0, 0}) {
		t.Errorf("degenerate normal = %v, want {0,1,0} or {0,0,0}", n)
	}
}

// --- float32ToByteColor ---

func TestFloat32ToByteColor(t *testing.T) {
	c := &TilesObjToMst{}
	result := c.float32ToByteColor([]float32{1.0, 0.5, 0.0})
	if result[0] != 255 || result[2] != 0 {
		t.Errorf("color = %v, want {255,*,0}", result)
	}
	if result[1] != 127 && result[1] != 128 {
		t.Errorf("green = %d, want 127 or 128", result[1])
	}
}

func TestFloat32ToByteColor_Empty(t *testing.T) {
	c := &TilesObjToMst{}
	result := c.float32ToByteColor(nil)
	if result != [3]byte{255, 255, 255} {
		t.Errorf("empty color = %v, want {255,255,255}", result)
	}
}

func TestFloat32ToByteColor_Overshoot(t *testing.T) {
	c := &TilesObjToMst{}
	result := c.float32ToByteColor([]float32{2.0, -0.5, 1.5})
	if result[0] != 254 || result[1] != 129 || result[2] != 126 {
		t.Errorf("overshoot = %v, want {254,129,126} (byte truncation)", result)
	}
}

// --- triangulateFace ---

func TestTriangulateFace_Triangle(t *testing.T) {
	c := &TilesObjToMst{}
	face := gobj.Face{Corners: []gobj.FaceCorner{{}, {}, {}}}
	tris := c.triangulateFace(face)
	if len(tris) != 1 {
		t.Errorf("triangle face: got %d triangles, want 1", len(tris))
	}
}

func TestTriangulateFace_Quad(t *testing.T) {
	c := &TilesObjToMst{}
	face := gobj.Face{Corners: []gobj.FaceCorner{{}, {}, {}, {}}}
	tris := c.triangulateFace(face)
	if len(tris) != 2 {
		t.Errorf("quad face: got %d triangles, want 2", len(tris))
	}
}

func TestTriangulateFace_Pentagon(t *testing.T) {
	c := &TilesObjToMst{}
	face := gobj.Face{Corners: []gobj.FaceCorner{{}, {}, {}, {}, {}}}
	tris := c.triangulateFace(face)
	if len(tris) != 3 {
		t.Errorf("pentagon face: got %d triangles, want 3", len(tris))
	}
}

func TestTriangulateFace_LessThan3(t *testing.T) {
	c := &TilesObjToMst{}
	face := gobj.Face{Corners: []gobj.FaceCorner{{}, {}}}
	tris := c.triangulateFace(face)
	if len(tris) != 0 {
		t.Errorf("line face: got %d triangles, want 0", len(tris))
	}
}

// --- parseMetadata ---

func TestTilesObj_ParseMetadata_FileNotFound(t *testing.T) {
	c := &TilesObjToMst{}
	err := c.parseMetadata("/nonexistent/metadata.xml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestTilesObj_ParseMetadata_WithTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.xml")
	content := `<?xml version="1.0" encoding="UTF-8"?>
<ModelMetadata version="1">
	<SRS>EPSG:4548</SRS>
	<SRSOrigin>518078.000000,4080366.000000,0.000000</SRSOrigin>
</ModelMetadata>`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	c := &TilesObjToMst{}
	if err := c.parseMetadata(path); err != nil {
		t.Fatalf("parseMetadata failed: %v", err)
	}
	if c.srs != "EPSG:4548" {
		t.Errorf("SRS = %q, want EPSG:4548", c.srs)
	}
	if c.origin != [3]float64{518078, 4080366, 0} {
		t.Errorf("origin = %v, want [518078 4080366 0]", c.origin)
	}
}

// --- computeGeoRef ---

func TestTilesObj_ComputeGeoRef_EmptySRS(t *testing.T) {
	c := &TilesObjToMst{}
	if ref := c.computeGeoRef(); ref != nil {
		t.Error("expected nil for empty SRS")
	}
}

func TestTilesObj_ComputeGeoRef_UnknownSRS(t *testing.T) {
	c := &TilesObjToMst{srs: "unknown"}
	if ref := c.computeGeoRef(); ref != nil {
		t.Error("expected nil for unknown SRS")
	}
}

func TestTilesObj_ComputeGeoRef_Preset(t *testing.T) {
	c := &TilesObjToMst{}
	if ref := c.computeGeoRef(); ref != nil {
		t.Error("expected nil for empty SRS")
	}
}

// --- Convert / ConvertMultiple ---

func TestTilesObjToMst_Convert_FileNotFound(t *testing.T) {
	c := &TilesObjToMst{}
	mesh, bbox, err := c.Convert("/nonexistent/file.obj")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if mesh != nil {
		t.Error("expected nil mesh on error")
	}
	_ = bbox
}

func TestTilesObjToMst_Convert_NoDataDir(t *testing.T) {
	c := &TilesObjToMst{}
	meshes, bbox, err := c.ConvertMultiple("/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
	if meshes != nil {
		t.Error("expected nil meshes on error")
	}
	_ = bbox
}

// --- Default values ---

func TestTilesObjToMst_DefaultValues(t *testing.T) {
	c := &TilesObjToMst{}
	if c.ApplyOrigin != false {
		t.Error("ApplyOrigin should default to false")
	}
}

// --- interface satisfaction ---

func TestTilesObjToMst_ImplementsFormatConvert(t *testing.T) {
	c := &TilesObjToMst{}
	if _, ok := interface{}(c).(FormatConvert); !ok {
		t.Error("TilesObjToMst does not implement FormatConvert")
	}
}

// --- ReadTileOrigin ---

func TestReadTileOrigin_NoFile(t *testing.T) {
	_, err := ReadTileOrigin("/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}
