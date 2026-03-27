package main

import (
	"fmt"
	"os"
	"path/filepath"

	mst "github.com/flywave/go-mst"

	"github.com/flywave/go-3dasset"
)

func findObjDirs(dataDir string) []string {
	var objDirs []string

	filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == "OBJ" {
			metadataPath := filepath.Join(path, "metadata.xml")
			if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
				return nil
			}
			objDirs = append(objDirs, path)
		}

		return nil
	})

	return objDirs
}

func processObjDir(objDir string) error {
	fmt.Printf("Processing: %s\n", objDir)

	converter := &asset3d.TilesObjToMst{ApplyOrigin: true}
	meshes, _, err := converter.ConvertMultiple(objDir)
	if err != nil {
		return fmt.Errorf("ConvertMultiple failed: %w", err)
	}

	if len(meshes) == 0 {
		return fmt.Errorf("no mesh generated")
	}

	if len(meshes) == 1 {
		mstPath := filepath.Join(filepath.Dir(objDir), "tiles.mst")
		err = mst.MeshWriteTo(mstPath, meshes[0])
		if err != nil {
			return fmt.Errorf("MeshWriteTo failed: %w", err)
		}
		fmt.Printf("  Saved to: %s\n", mstPath)
		return nil
	}

	mergedMesh := mst.NewMesh()
	for i := range meshes {
		offset := uint32(len(mergedMesh.Materials))
		for _, node := range meshes[i].Nodes {
			for _, fg := range node.FaceGroup {
				fg.Batchid += int32(offset)
			}
		}
		mergedMesh.Materials = append(mergedMesh.Materials, meshes[i].Materials...)
		mergedMesh.Nodes = append(mergedMesh.Nodes, meshes[i].Nodes...)
	}

	mstPath := filepath.Join(filepath.Dir(objDir), "tiles.mst")
	err = mst.MeshWriteTo(mstPath, mergedMesh)
	if err != nil {
		return fmt.Errorf("MeshWriteTo failed: %w", err)
	}

	fmt.Printf("  Saved to: %s (%d tiles merged)\n", mstPath, len(meshes))
	return nil
}

func main() {
	dataDirs := []string{"./data/1/0131/Model"}

	for _, dataDir := range dataDirs {
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			fmt.Printf("Skipping %s (not found)\n", dataDir)
			continue
		}

		objDirs := findObjDirs(dataDir)
		if len(objDirs) == 0 {
			fmt.Printf("No OBJ directories found in %s\n", dataDir)
			continue
		}

		fmt.Printf("Found %d OBJ directories in %s\n", len(objDirs), dataDir)

		for _, objDir := range objDirs {
			if err := processObjDir(objDir); err != nil {
				fmt.Printf("  Error: %v\n", err)
			}
		}
	}

	fmt.Println("Done!")
}
