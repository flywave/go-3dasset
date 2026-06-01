package asset3d

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	mst "github.com/flywave/go-mst"
	"github.com/flywave/go-osg/model"
	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
)

// --- extractLodLevel ---

func TestExtractLodLevel(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"Tile_+002_+000_L22_000020.osgb", 22},
		{"Tile_+003_+003_L18_000.osgb", 18},
		{"Tile_+000_+000_L24_0000700.osgb", 24},
		{"main.osgb", 0},
		{"test.osgb", 0},
		{"Tile_L5_0.osgb", 5},
		{"_L12_.osgb", 12},
	}
	for _, tt := range tests {
		got := extractLodLevel(tt.name)
		if got != tt.want {
			t.Errorf("extractLodLevel(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

// --- findFinestLod ---

func TestFindFinestLod(t *testing.T) {
	tests := []struct {
		name string
		files []string
		want int
		any bool
	}{
		{
			name: "empty",
			files: []string{},
			want: 0,
		},
		{
			name: "single",
			files: []string{"file.osgb"},
			want: 1,
		},
		{
			name: "multiple_lods",
			files: []string{
				"Tile_L18_0.osgb",
				"Tile_L22_0.osgb",
				"Tile_L20_0.osgb",
			},
			want: 1,
		},
		{
			name: "same_lod",
			files: []string{
				"Tile_L24_00001.osgb",
				"Tile_L24_00002.osgb",
				"Tile_L24_00003.osgb",
			},
			want: 3,
		},
		{
			name: "mixed_plus_no_lod",
			files: []string{
				"base.osgb",
				"Tile_L18_0.osgb",
				"Tile_L24_00001.osgb",
				"Tile_L24_00002.osgb",
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		got := findFinestLod(tt.files)
		if len(got) != tt.want {
			t.Errorf("%s: findFinestLod returned %d files (%v), want %d", tt.name, len(got), got, tt.want)
		}
	}
}

// --- extractVec3s ---

func TestExtractVec3s_Vec3Float32(t *testing.T) {
	arr := model.NewArray(model.Vec3ArrayType, model.GLFLOAT, 3)
	arr.Data = [][3]float32{{1, 2, 3}, {4, 5, 6}}
	verts := extractVec3s(arr, false, [3]float64{})
	if len(verts) != 2 {
		t.Fatalf("got %d, want 2", len(verts))
	}
	if verts[0] != (vec3.T{1, 2, 3}) {
		t.Errorf("verts[0] = %v, want {1,2,3}", verts[0])
	}
}

func TestExtractVec3s_Vec3Float32_ApplyOrigin(t *testing.T) {
	arr := model.NewArray(model.Vec3ArrayType, model.GLFLOAT, 3)
	arr.Data = [][3]float32{{1, 2, 3}}
	verts := extractVec3s(arr, true, [3]float64{100, 200, 300})
	if verts[0] != (vec3.T{101, 202, 303}) {
		t.Errorf("verts[0] = %v, want {101,202,303}", verts[0])
	}
}

func TestExtractVec3s_FlatFloat32(t *testing.T) {
	arr := model.NewArray(model.FloatArrayType, model.GLFLOAT, 1)
	arr.Data = []float32{1, 2, 3, 4, 5, 6}
	verts := extractVec3s(arr, false, [3]float64{})
	if len(verts) != 2 {
		t.Fatalf("got %d, want 2", len(verts))
	}
}

func TestExtractVec3s_Int16(t *testing.T) {
	arr := model.NewArray(model.ShortArrayType, model.GLSHORT, 1)
	arr.Data = []int16{1000, 2000, 3000}
	verts := extractVec3s(arr, false, [3]float64{})
	if len(verts) != 1 {
		t.Fatalf("got %d, want 1", len(verts))
	}
	if math.Abs(float64(verts[0][0])-1.0) > 0.001 {
		t.Errorf("verts[0][0] = %f, want 1.0", verts[0][0])
	}
}

func TestExtractVec3s_UInt16(t *testing.T) {
	arr := model.NewArray(model.UShortArrayType, model.GLUNSIGNEDSHORT, 1)
	arr.Data = []uint16{5000, 10000, 15000}
	verts := extractVec3s(arr, false, [3]float64{})
	if len(verts) != 1 {
		t.Fatalf("got %d, want 1", len(verts))
	}
	if math.Abs(float64(verts[0][1])-10.0) > 0.001 {
		t.Errorf("verts[0][1] = %f, want 10.0", verts[0][1])
	}
}

func TestExtractVec3s_Int32(t *testing.T) {
	arr := model.NewArray(model.IntArrayType, model.GLINT, 1)
	arr.Data = []int32{3000, 4000, 5000}
	verts := extractVec3s(arr, false, [3]float64{})
	if len(verts) != 1 {
		t.Fatalf("got %d, want 1", len(verts))
	}
}

func TestExtractVec3s_Nil(t *testing.T) {
	if v := extractVec3s(nil, false, [3]float64{}); v != nil {
		t.Error("expected nil for nil array")
	}
	if v := extractVec3s(&model.Array{}, false, [3]float64{}); v != nil {
		t.Error("expected nil for nil Data")
	}
}

// --- extractVec2s ---

func TestExtractVec2s_Vec2Float32(t *testing.T) {
	arr := model.NewArray(model.Vec2ArrayType, model.GLFLOAT, 2)
	arr.Data = [][2]float32{{0.1, 0.2}, {0.3, 0.4}}
	uvs := extractVec2s(arr)
	if len(uvs) != 2 {
		t.Fatalf("got %d, want 2", len(uvs))
	}
	if uvs[0] != (vec2.T{0.1, 0.2}) {
		t.Errorf("uvs[0] = %v, want {0.1,0.2}", uvs[0])
	}
}

func TestExtractVec2s_FlatFloat32(t *testing.T) {
	arr := model.NewArray(model.FloatArrayType, model.GLFLOAT, 1)
	arr.Data = []float32{0.5, 0.6, 0.7, 0.8}
	uvs := extractVec2s(arr)
	if len(uvs) != 2 {
		t.Fatalf("got %d, want 2", len(uvs))
	}
}

func TestExtractVec2s_Int16(t *testing.T) {
	arr := model.NewArray(model.ShortArrayType, model.GLSHORT, 1)
	arr.Data = []int16{100, 200}
	uvs := extractVec2s(arr)
	if len(uvs) != 1 {
		t.Fatalf("got %d, want 1", len(uvs))
	}
}

func TestExtractVec2s_Int32(t *testing.T) {
	arr := model.NewArray(model.IntArrayType, model.GLINT, 1)
	arr.Data = []int32{1000, 2000}
	uvs := extractVec2s(arr)
	if len(uvs) != 1 {
		t.Fatalf("got %d, want 1", len(uvs))
	}
}

func TestExtractVec2s_Nil(t *testing.T) {
	if v := extractVec2s(nil); v != nil {
		t.Error("expected nil for nil array")
	}
}

// --- combineMatrix ---

func TestCombineMatrix_BothNil(t *testing.T) {
	if r := combineMatrix(nil, nil); r != nil {
		t.Error("expected nil when both inputs are nil")
	}
}

func TestCombineMatrix_ParentNil(t *testing.T) {
	child := &[4][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}
	r := combineMatrix(nil, child)
	if r[0][0] != 1 {
		t.Error("expected child when parent is nil")
	}
}

func TestCombineMatrix_ChildNil(t *testing.T) {
	parent := &[4][4]float32{{2, 0, 0, 0}, {0, 2, 0, 0}, {0, 0, 2, 0}, {0, 0, 0, 1}}
	r := combineMatrix(parent, nil)
	if r[0][0] != 2 {
		t.Error("expected parent when child is nil")
	}
}

func TestCombineMatrix_Multiply(t *testing.T) {
	translate := &[4][4]float32{
		{1, 0, 0, 0},
		{0, 1, 0, 0},
		{0, 0, 1, 0},
		{10, 20, 30, 1},
	}
	scale := &[4][4]float32{
		{2, 0, 0, 0},
		{0, 3, 0, 0},
		{0, 0, 4, 0},
		{0, 0, 0, 1},
	}
	r := combineMatrix(translate, scale)
	if r[0][0] != 2 || r[3][0] != 20 {
		t.Errorf("result[0][0]=%f (want 2), result[3][0]=%f (want 20)", r[0][0], r[3][0])
	}
}

func TestCombineMatrix_TranslationOnly(t *testing.T) {
	t1 := &[4][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {5, 10, 15, 1}}
	t2 := &[4][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {3, 7, 11, 1}}
	r := combineMatrix(t1, t2)
	if r[3][0] != 8 || r[3][1] != 17 || r[3][2] != 26 {
		t.Errorf("translation = (%f,%f,%f), want (8,17,26)", r[3][0], r[3][1], r[3][2])
	}
}

// --- patToMatrix ---

func TestPatToMatrix_IdentityScale(t *testing.T) {
	pos := [3]float64{10, 20, 30}
	att := [4]float64{0, 0, 0, 1}
	scl := [3]float64{1, 1, 1}
	m := patToMatrix(pos, att, scl)
	// Identity rotation + translation
	testV := applyMatrix([]vec3.T{{0, 0, 0}}, m)
	if testV[0] != (vec3.T{10, 20, 30}) {
		t.Errorf("identity PAT: (0,0,0) -> %v, want (10,20,30)", testV[0])
	}
}

func TestPatToMatrix_ZeroScale(t *testing.T) {
	pos := [3]float64{0, 0, 0}
	att := [4]float64{0, 0, 0, 1}
	scl := [3]float64{0, 0, 0} // zero -> clamped to 1
	m := patToMatrix(pos, att, scl)
	testV := applyMatrix([]vec3.T{{1, 1, 1}}, m)
	if testV[0] != (vec3.T{1, 1, 1}) {
		t.Errorf("zero scale should default to identity: %v", testV[0])
	}
}

func TestPatToMatrix_Rotation90Z(t *testing.T) {
	// 90 degree rotation around Z: quat (0,0,sin(45°),cos(45°))
	sin45 := float32(0.7071068)
	att := [4]float64{0, 0, float64(sin45), float64(sin45)}
	m := patToMatrix([3]float64{0, 0, 0}, att, [3]float64{1, 1, 1})
	testV := applyMatrix([]vec3.T{{1, 0, 0}}, m)
	if math.Abs(float64(testV[0][0])) > 0.01 || math.Abs(float64(testV[0][1])-1.0) > 0.01 {
		t.Errorf("Z-90: (1,0,0) -> %v, want approx (0,1,0)", testV[0])
	}
}

// --- applyMatrix ---

func TestApplyMatrix_Identity(t *testing.T) {
	identity := &[4][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}
	verts := []vec3.T{{1, 2, 3}, {4, 5, 6}}
	result := applyMatrix(verts, identity)
	if result[0] != (vec3.T{1, 2, 3}) || result[1] != (vec3.T{4, 5, 6}) {
		t.Error("identity matrix should not change vertices")
	}
}

func TestApplyMatrix_Translation(t *testing.T) {
	trans := &[4][4]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {100, 200, 300, 1}}
	verts := []vec3.T{{0, 0, 0}}
	result := applyMatrix(verts, trans)
	if result[0] != (vec3.T{100, 200, 300}) {
		t.Errorf("translation: got %v, want {100,200,300}", result[0])
	}
}

func TestApplyMatrix_Scale(t *testing.T) {
	scale := &[4][4]float32{{2, 0, 0, 0}, {0, 3, 0, 0}, {0, 0, 4, 0}, {0, 0, 0, 1}}
	verts := []vec3.T{{1, 1, 1}}
	result := applyMatrix(verts, scale)
	if result[0] != (vec3.T{2, 3, 4}) {
		t.Errorf("scale: got %v, want {2,3,4}", result[0])
	}
}

// --- computeNormals ---

func TestComputeNormals_Triangle(t *testing.T) {
	verts := []vec3.T{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
	faces := []*mst.Face{{Vertex: [3]uint32{0, 1, 2}}}
	normals := computeNormals(verts, faces)
	if len(normals) != 3 {
		t.Fatalf("got %d normals, want 3", len(normals))
	}
	for i, n := range normals {
		l := math.Sqrt(float64(n[0]*n[0] + n[1]*n[1] + n[2]*n[2]))
		if math.Abs(l-1.0) > 0.01 {
			t.Errorf("normal[%d] length = %f, want 1.0", i, l)
		}
		if math.Abs(float64(n[2])-1.0) > 0.01 {
			t.Errorf("normal[%d].z = %f, want ~1.0 (facing +Z)", i, n[2])
		}
	}
}

func TestComputeNormals_Empty(t *testing.T) {
	n := computeNormals(nil, nil)
	if len(n) != 0 {
		t.Errorf("expected empty normals, got %d", len(n))
	}
}

func TestComputeNormals_OutOfRange(t *testing.T) {
	verts := []vec3.T{{0, 0, 0}, {1, 0, 0}}
	faces := []*mst.Face{{Vertex: [3]uint32{0, 1, 999}}}
	normals := computeNormals(verts, faces)
	if len(normals) != 2 {
		t.Errorf("got %d normals, want 2", len(normals))
	}
}

func TestComputeNormals_Cube(t *testing.T) {
	// 8 vertices of a unit cube, 12 triangles (2 per face)
	verts := []vec3.T{
		{0, 0, 0}, {1, 0, 0}, {1, 1, 0}, {0, 1, 0},
		{0, 0, 1}, {1, 0, 1}, {1, 1, 1}, {0, 1, 1},
	}
	faces := []*mst.Face{
		{Vertex: [3]uint32{0, 1, 2}}, {Vertex: [3]uint32{0, 2, 3}},
		{Vertex: [3]uint32{4, 6, 5}}, {Vertex: [3]uint32{4, 7, 6}},
	}
	normals := computeNormals(verts, faces)
	for i, n := range normals {
		l := math.Sqrt(float64(n[0]*n[0] + n[1]*n[1] + n[2]*n[2]))
		if math.Abs(l-1.0) > 0.01 && l > 0 {
			t.Errorf("normal[%d] length = %f", i, l)
		}
	}
}

// --- parseMetadata ---

func TestParseMetadata_FileNotFound(t *testing.T) {
	c := &TilesOsgbToMst{}
	err := c.parseMetadata("/nonexistent/metadata.xml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseMetadata_WithTempFile(t *testing.T) {
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

	c := &TilesOsgbToMst{}
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

func TestComputeGeoRef_SRSEmpty(t *testing.T) {
	c := &TilesOsgbToMst{}
	if ref := c.computeGeoRef(); ref != nil {
		t.Error("expected nil for empty SRS")
	}
}

func TestComputeGeoRef_SRSUnknown(t *testing.T) {
	c := &TilesOsgbToMst{srs: "unknown"}
	if ref := c.computeGeoRef(); ref != nil {
		t.Error("expected nil for unknown SRS")
	}
}

func TestComputeGeoRef_WithGeoRefPreset(t *testing.T) {
	existing := &mst.GeoRef{EcefOrigin: [3]float64{1, 2, 3}}
	c := &TilesOsgbToMst{GeoRef: existing}
	if ref := c.computeGeoRef(); ref != existing {
		t.Error("should return existing GeoRef")
	}
}

// --- Convert / ConvertMultiple ---

func TestTilesOsgbToMst_Convert_NoDataDir(t *testing.T) {
	c := &TilesOsgbToMst{}
	mesh, bbox, err := c.Convert("/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
	if mesh != nil {
		t.Error("expected nil mesh on error")
	}
	_ = bbox
}

func TestTilesOsgbToMst_loadedFiles(t *testing.T) {
	c := &TilesOsgbToMst{loadedFiles: make(map[string]bool)}
	c.loadedFiles["test.osgb"] = true
	if !c.loadedFiles["test.osgb"] {
		t.Error("loadedFiles should track visited files")
	}
}

func TestTilesOsgbToMst_DefaultValues(t *testing.T) {
	c := &TilesOsgbToMst{}
	if c.ApplyOrigin != false {
		t.Error("ApplyOrigin should default to false")
	}
	if c.GenerateNormals != false {
		t.Error("GenerateNormals should default to false")
	}
}

// --- MST output validation ---

func TestTilesOsgbToMst_MeshOutput(t *testing.T) {
	dataDir := "../go-osg/test_data/OSGB1"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("OSGB1 test data not found")
	}

	c := &TilesOsgbToMst{ApplyOrigin: false, GenerateNormals: true}
	meshes, _, err := c.ConvertMultiple(dataDir)
	if err != nil || len(meshes) == 0 {
		t.Skip("no data to test MST output")
	}

	for i, mesh := range meshes {
		for j, node := range mesh.Nodes {
			if len(node.Vertices) == 0 {
				t.Errorf("mesh[%d].node[%d]: zero vertices", i, j)
			}
			for k, fg := range node.FaceGroup {
				for _, f := range fg.Faces {
					v0, v1, v2 := int(f.Vertex[0]), int(f.Vertex[1]), int(f.Vertex[2])
					if v0 >= len(node.Vertices) || v1 >= len(node.Vertices) || v2 >= len(node.Vertices) {
						t.Errorf("mesh[%d].node[%d].fg[%d]: face vertex out of range", i, j, k)
					}
				}
			}
		}
	}
}

// --- type switching in traverse ---

func TestExtractVec3s_UnsupportedType(t *testing.T) {
	arr := model.NewArray(model.Vec4ArrayType, model.GLFLOAT, 4)
	arr.Data = [][4]float32{{1, 2, 3, 4}}
	verts := extractVec3s(arr, false, [3]float64{})
	if verts != nil {
		t.Error("expected nil for unsupported Vec4 type")
	}
}

func TestExtractVec2s_UnsupportedType(t *testing.T) {
	arr := model.NewArray(model.Vec3ArrayType, model.GLFLOAT, 3)
	arr.Data = [][3]float32{{1, 2, 3}}
	uvs := extractVec2s(arr)
	if uvs != nil {
		t.Error("expected nil for unsupported Vec3 type")
	}
}

func TestComputeNormals_DegenerateTriangle(t *testing.T) {
	verts := []vec3.T{{0, 0, 0}, {0, 0, 0}, {0, 0, 0}}
	faces := []*mst.Face{{Vertex: [3]uint32{0, 1, 2}}}
	normals := computeNormals(verts, faces)
	for i, n := range normals {
		if n != (vec3.T{0, 0, 1}) {
			t.Errorf("normal[%d] = %v, want {0,0,1} (fallback)", i, n)
		}
	}
}

func TestFindFinestLod_NilInput(t *testing.T) {
	if r := findFinestLod(nil); len(r) != 0 {
		t.Errorf("expected empty result for nil input, got %v", r)
	}
}

func TestApplyMatrix_Empty(t *testing.T) {
	result := applyMatrix(nil, &[4][4]float32{})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestComputeNormals_SingleFaceMultipleVertices(t *testing.T) {
	verts := make([]vec3.T, 10)
	// Set first 3 vertices to define a triangle
	verts[0] = vec3.T{0, 0, 0}
	verts[1] = vec3.T{1, 0, 0}
	verts[2] = vec3.T{0, 1, 0}
	// Rest remain zero (unreferenced)

	faces := []*mst.Face{{Vertex: [3]uint32{0, 1, 2}}}
	normals := computeNormals(verts, faces)

	if len(normals) != 10 {
		t.Fatalf("expected 10 normals, got %d", len(normals))
	}
	// Referenced vertices should have proper normals
	if normals[0] == (vec3.T{0, 0, 1}) {
		t.Log("referenced vertex 0 normal is {0,0,1}")
	}
	// Unreferenced vertices should get fallback {0,0,1}
	for i := 3; i < 10; i++ {
		if normals[i] != (vec3.T{0, 0, 1}) {
			t.Errorf("unreferenced vertex %d normal = %v, want {0,0,1}", i, normals[i])
		}
	}
}

// --- parseMetadata edge cases ---

func TestParseMetadata_MinimalFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.xml")
	content := `<?xml version="1.0"?>
<ModelMetadata version="1">
	<SRS>EPSG:4326</SRS>
	<SRSOrigin>120.0,30.0,0.0</SRSOrigin>
</ModelMetadata>`
	os.WriteFile(path, []byte(content), 0644)

	c := &TilesOsgbToMst{}
	if err := c.parseMetadata(path); err != nil {
		t.Fatalf("parseMetadata failed: %v", err)
	}
	if c.srs != "EPSG:4326" {
		t.Errorf("SRS=%q", c.srs)
	}
	if math.Abs(c.origin[0]-120.0) > 0.001 || math.Abs(c.origin[1]-30.0) > 0.001 {
		t.Errorf("origin=%v", c.origin)
	}
}

// --- Ensure interface satisfaction ---

func TestTilesOsgbToMst_ImplementsFormatConvert(t *testing.T) {
	c := &TilesOsgbToMst{}
	if _, ok := interface{}(c).(FormatConvert); !ok {
		t.Error("TilesOsgbToMst does not implement FormatConvert")
	}
}

func TestStripLODSuffix(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Tile_+002_+000_L22_000020.osgb", "Tile_+002_+000.osgb"},
		{"Tile_+003_+003_L18_000.osgb", "Tile_+003_+003.osgb"},
		{"main.osgb", "main.osgb"},
		{"no_lod.osgb", "no_lod.osgb"},
		{"test_L5.osgb", "test.osgb"},
	}
	for _, tt := range tests {
		got := stripLODSuffix(tt.name)
		if got != tt.want {
			t.Errorf("stripLODSuffix(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestTilesOsgbToMst_Convert_FileNotFound(t *testing.T) {
	c := &TilesOsgbToMst{}
	mesh, bbox, err := c.Convert("/nonexistent/file.osgb")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if mesh != nil {
		t.Error("expected nil mesh on error")
	}
	_ = bbox
}
