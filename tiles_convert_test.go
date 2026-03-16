package asset3d

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTilesObjConvert(t *testing.T) {
	dataDir := "./data/1/0131/Model/OBJ"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("Test data not found")
		return
	}

	origin, err := ReadTileOrigin(dataDir)
	if err != nil {
		t.Fatalf("ReadTileOrigin failed: %v", err)
	}
	t.Logf("Origin: %v", origin)

	converter := &TilesObjToMst{ApplyOrigin: false}
	start := time.Now()
	mesh, bbox, err := converter.Convert(dataDir)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	t.Logf("Without origin - BBox: %v, Nodes: %d, Materials: %d, Duration: %v",
		bbox, len(mesh.Nodes), len(mesh.Materials), time.Since(start))

	outputDir := "./test_output"
	os.MkdirAll(outputDir, 0755)
	glbPath := filepath.Join(outputDir, "tiles_obj_local.glb")
	start = time.Now()
	err = ConvertToGlb(mesh, glbPath)
	if err != nil {
		t.Fatalf("ConvertToGlb failed: %v", err)
	}
	t.Logf("GLB saved to %s, Duration: %v", glbPath, time.Since(start))

	stat, err := os.Stat(glbPath)
	if err != nil {
		t.Fatalf("GLB file not found: %v", err)
	}
	t.Logf("GLB file size: %d bytes", stat.Size())
}

func TestTilesObjConvertWithOrigin(t *testing.T) {
	dataDir := "./data/1/0131/Model/OBJ"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("Test data not found")
		return
	}

	origin, err := ReadTileOrigin(dataDir)
	if err != nil {
		t.Fatalf("ReadTileOrigin failed: %v", err)
	}
	t.Logf("Origin: %v", origin)

	converter := &TilesObjToMst{ApplyOrigin: true}
	start := time.Now()
	mesh, bbox, err := converter.Convert(dataDir)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	t.Logf("With origin - BBox: %v, Nodes: %d, Materials: %d, Duration: %v",
		bbox, len(mesh.Nodes), len(mesh.Materials), time.Since(start))

	outputDir := "./test_output"
	os.MkdirAll(outputDir, 0755)
	glbPath := filepath.Join(outputDir, "tiles_obj_world.glb")
	start = time.Now()
	err = ConvertToGlb(mesh, glbPath)
	if err != nil {
		t.Fatalf("ConvertToGlb failed: %v", err)
	}
	t.Logf("GLB saved to %s, Duration: %v", glbPath, time.Since(start))

	stat, err := os.Stat(glbPath)
	if err != nil {
		t.Fatalf("GLB file not found: %v", err)
	}
	t.Logf("GLB file size: %d bytes", stat.Size())
}

func TestTilesOsgbConvert(t *testing.T) {
	dataDir := "./data/1/0131/Model/OSGB"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("Test data not found")
		return
	}

	origin, err := ReadTileOrigin(dataDir)
	if err != nil {
		t.Fatalf("ReadTileOrigin failed: %v", err)
	}
	t.Logf("Origin: %v", origin)

	converter := &TilesOsgbToMst{ApplyOrigin: false}
	start := time.Now()
	mesh, bbox, err := converter.Convert(dataDir)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	t.Logf("Without origin - BBox: %v, Nodes: %d, Materials: %d, Duration: %v",
		bbox, len(mesh.Nodes), len(mesh.Materials), time.Since(start))

	outputDir := "./test_output"
	os.MkdirAll(outputDir, 0755)
	glbPath := filepath.Join(outputDir, "tiles_osgb_local.glb")
	start = time.Now()
	err = ConvertToGlb(mesh, glbPath)
	if err != nil {
		t.Fatalf("ConvertToGlb failed: %v", err)
	}
	t.Logf("GLB saved to %s, Duration: %v", glbPath, time.Since(start))

	stat, err := os.Stat(glbPath)
	if err != nil {
		t.Fatalf("GLB file not found: %v", err)
	}
	t.Logf("GLB file size: %d bytes", stat.Size())
}

func TestTilesOsgbConvertWithOrigin(t *testing.T) {
	dataDir := "./data/1/0131/Model/OSGB"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("Test data not found")
		return
	}

	origin, err := ReadTileOrigin(dataDir)
	if err != nil {
		t.Fatalf("ReadTileOrigin failed: %v", err)
	}
	t.Logf("Origin: %v", origin)

	converter := &TilesOsgbToMst{ApplyOrigin: true}
	start := time.Now()
	mesh, bbox, err := converter.Convert(dataDir)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	t.Logf("With origin - BBox: %v, Nodes: %d, Materials: %d, Duration: %v",
		bbox, len(mesh.Nodes), len(mesh.Materials), time.Since(start))

	outputDir := "./test_output"
	os.MkdirAll(outputDir, 0755)
	glbPath := filepath.Join(outputDir, "tiles_osgb_world.glb")
	start = time.Now()
	err = ConvertToGlb(mesh, glbPath)
	if err != nil {
		t.Fatalf("ConvertToGlb failed: %v", err)
	}
	t.Logf("GLB saved to %s, Duration: %v", glbPath, time.Since(start))

	stat, err := os.Stat(glbPath)
	if err != nil {
		t.Fatalf("GLB file not found: %v", err)
	}
	t.Logf("GLB file size: %d bytes", stat.Size())
}
