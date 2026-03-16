package asset3d

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	mst "github.com/flywave/go-mst"
	osg "github.com/flywave/go-osg"
	"github.com/flywave/go-osg/model"
	vec3d "github.com/flywave/go3d/float64/vec3"

	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
)

type TilesOsgbToMst struct {
	currentPath string
	dataPath    string
	origin      [3]float64
	loadedFiles map[string]bool
	ApplyOrigin bool
}

func (t *TilesOsgbToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	t.currentPath = path
	t.loadedFiles = make(map[string]bool)

	metadataPath := filepath.Join(path, "metadata.xml")
	if err := t.parseMetadata(metadataPath); err != nil {
		return nil, nil, err
	}

	t.dataPath = filepath.Join(path, "Data")
	if _, err := os.Stat(t.dataPath); os.IsNotExist(err) {
		return nil, nil, err
	}

	mesh := mst.NewMesh()
	ext := vec3d.MinBox

	mainFile := filepath.Join(t.dataPath, "main.osgb")
	if _, err := os.Stat(mainFile); err == nil {
		t.processOsgbFile(mainFile, mesh, &ext)
	}

	tileDirs, err := filepath.Glob(filepath.Join(t.dataPath, "Tile_*"))
	if err == nil {
		for _, tileDir := range tileDirs {
			osgbFiles, err := filepath.Glob(filepath.Join(tileDir, "*.osgb"))
			if err != nil {
				continue
			}
			finestFile := t.findFinestLod(osgbFiles)
			if finestFile != "" {
				t.processOsgbFile(finestFile, mesh, &ext)
			}
		}
	}

	return mesh, ext.Array(), nil
}

func (t *TilesOsgbToMst) findFinestLod(files []string) string {
	if len(files) == 0 {
		return ""
	}
	if len(files) == 1 {
		return files[0]
	}

	maxLod := -1
	var finestFile string

	for _, f := range files {
		base := filepath.Base(f)
		lod := t.extractLodLevel(base)
		if lod > maxLod {
			maxLod = lod
			finestFile = f
		}
	}

	return finestFile
}

func (t *TilesOsgbToMst) extractLodLevel(filename string) int {
	lod := 0
	for i := 0; i < len(filename); i++ {
		if filename[i] == 'L' && i+1 < len(filename) {
			j := i + 1
			for j < len(filename) && filename[j] >= '0' && filename[j] <= '9' {
				lod = lod*10 + int(filename[j]-'0')
				j++
			}
			return lod
		}
	}
	return lod
}

func (t *TilesOsgbToMst) parseMetadata(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var metadata ModelMetadata
	if err := xml.Unmarshal(data, &metadata); err != nil {
		return err
	}

	parts := strings.Split(metadata.SRSOrigin, ",")
	if len(parts) >= 3 {
		t.origin[0] = parseFloat(parts[0])
		t.origin[1] = parseFloat(parts[1])
		t.origin[2] = parseFloat(parts[2])
	}

	return nil
}

func (t *TilesOsgbToMst) processOsgbFile(osgbPath string, mesh *mst.Mesh, ext *vec3d.Box) error {
	defer func() {
		if r := recover(); r != nil {
			// log the error and		}
		}
	}

	absPath, _ := filepath.Abs(osgbPath)
	if t.loadedFiles[absPath] {
		return nil
	}
	t.loadedFiles[absPath] = true

	rw := osg.NewReadWrite()
	result := rw.ReadNode(osgbPath, nil)
	if result == nil || result.GetNode() == nil {
		return nil
	}

	node := result.GetNode()
	t.traverseNode(node, mesh, ext, filepath.Dir(osgbPath))

	return nil
}

func (t *TilesOsgbToMst) traverseNode(node model.NodeInterface, mesh *mst.Mesh, ext *vec3d.Box, baseDir string) {
	switch n := node.(type) {
	case *model.Group:
		for _, child := range n.Children {
			t.traverseNode(child, mesh, ext, baseDir)
		}
	case *model.PagedLod:
		for _, child := range n.Children {
			t.traverseNode(child, mesh, ext, baseDir)
		}
		for _, perData := range n.PerRangeDataList {
			if perData.FileName != "" {
				childPath := perData.FileName
				if !filepath.IsAbs(childPath) {
					childPath = filepath.Join(baseDir, childPath)
				}
				if _, err := os.Stat(childPath); err == nil {
					t.processOsgbFile(childPath, mesh, ext)
				}
			}
		}
	case *model.Geode:
		for _, child := range n.Children {
			if geom, ok := child.(*model.Geometry); ok {
				t.processGeometry(geom, mesh, ext)
			}
		}
	case *model.Geometry:
		t.processGeometry(n, mesh, ext)
	}
}

func (t *TilesOsgbToMst) processGeometry(geom *model.Geometry, mesh *mst.Mesh, ext *vec3d.Box) {
	if geom.VertexArray == nil || geom.VertexArray.Data == nil {
		return
	}

	vertices := t.extractVec3Array(geom.VertexArray)
	if len(vertices) == 0 {
		return
	}

	var texCoords []vec2.T
	if len(geom.TexCoordArrayList) > 0 && geom.TexCoordArrayList[0] != nil && geom.TexCoordArrayList[0].Data != nil {
		texCoords = t.extractVec2Array(geom.TexCoordArrayList[0])
	}

	meshNode := &mst.MeshNode{}
	faceGroup := &mst.MeshTriangle{Batchid: int32(len(mesh.Materials))}

	for _, prim := range geom.Primitives {
		t.processPrimitive(prim, vertices, texCoords, meshNode, faceGroup, ext)
	}

	if len(faceGroup.Faces) > 0 {
		meshNode.FaceGroup = append(meshNode.FaceGroup, faceGroup)
		mesh.Nodes = append(mesh.Nodes, meshNode)
		mesh.Materials = append(mesh.Materials, &mst.BaseMaterial{Color: [3]byte{200, 200, 200}})
	}
}

func (t *TilesOsgbToMst) extractVec3Array(arr *model.Array) []vec3.T {
	if arr == nil || arr.Data == nil {
		return nil
	}

	switch data := arr.Data.(type) {
	case []vec3.T:
		result := make([]vec3.T, len(data))
		for i, v := range data {
			if t.ApplyOrigin {
				result[i] = vec3.T{
					float32(float64(v[0]) + t.origin[0]),
					float32(float64(v[1]) + t.origin[1]),
					float32(float64(v[2]) + t.origin[2]),
				}
			} else {
				result[i] = v
			}
		}
		return result
	case []float32:
		count := len(data) / 3
		result := make([]vec3.T, count)
		for i := 0; i < count; i++ {
			if t.ApplyOrigin {
				result[i] = vec3.T{
					float32(float64(data[i*3]) + t.origin[0]),
					float32(float64(data[i*3+1]) + t.origin[1]),
					float32(float64(data[i*3+2]) + t.origin[2]),
				}
			} else {
				result[i] = vec3.T{
					data[i*3],
					data[i*3+1],
					data[i*3+2],
				}
			}
		}
		return result
	}
	return nil
}

func (t *TilesOsgbToMst) extractVec2Array(arr *model.Array) []vec2.T {
	if arr == nil || arr.Data == nil {
		return nil
	}

	switch data := arr.Data.(type) {
	case []vec2.T:
		return data
	case []float32:
		count := len(data) / 2
		result := make([]vec2.T, count)
		for i := 0; i < count; i++ {
			result[i] = vec2.T{data[i*2], data[i*2+1]}
		}
		return result
	}
	return nil
}

func (t *TilesOsgbToMst) processPrimitive(prim interface{}, vertices []vec3.T, texCoords []vec2.T, meshNode *mst.MeshNode, faceGroup *mst.MeshTriangle, ext *vec3d.Box) {
	switch p := prim.(type) {
	case *model.DrawElementsUShort:
		t.processDrawElementsUShort(p.Data, vertices, texCoords, meshNode, faceGroup, ext)
	case *model.DrawElementsUInt:
		t.processDrawElementsUInt(p.Data, vertices, texCoords, meshNode, faceGroup, ext)
	case *model.DrawArrays:
		t.processDrawArrays(p.First, p.Count, vertices, texCoords, meshNode, faceGroup, ext)
	}
}

func (t *TilesOsgbToMst) processDrawElementsUShort(indices []uint16, vertices []vec3.T, texCoords []vec2.T, meshNode *mst.MeshNode, faceGroup *mst.MeshTriangle, ext *vec3d.Box) {
	for i := 0; i+2 < len(indices); i += 3 {
		v0 := int(indices[i])
		v1 := int(indices[i+1])
		v2 := int(indices[i+2])

		if v0 >= len(vertices) || v1 >= len(vertices) || v2 >= len(vertices) {
			continue
		}

		baseIndex := uint32(len(meshNode.Vertices))

		for _, vi := range []int{v0, v1, v2} {
			v := vertices[vi]
			meshNode.Vertices = append(meshNode.Vertices, v)
			ext.Extend(&vec3d.T{float64(v[0]), float64(v[1]), float64(v[2])})

			if texCoords != nil && vi < len(texCoords) {
				meshNode.TexCoords = append(meshNode.TexCoords, texCoords[vi])
			} else {
				meshNode.TexCoords = append(meshNode.TexCoords, vec2.T{0, 0})
			}

			meshNode.Normals = append(meshNode.Normals, vec3.T{0, 0, 1})
		}

		faceGroup.Faces = append(faceGroup.Faces, &mst.Face{
			Vertex: [3]uint32{baseIndex, baseIndex + 1, baseIndex + 2},
		})
	}
}

func (t *TilesOsgbToMst) processDrawElementsUInt(indices []uint32, vertices []vec3.T, texCoords []vec2.T, meshNode *mst.MeshNode, faceGroup *mst.MeshTriangle, ext *vec3d.Box) {
	for i := 0; i+2 < len(indices); i += 3 {
		v0 := int(indices[i])
		v1 := int(indices[i+1])
		v2 := int(indices[i+2])

		if v0 >= len(vertices) || v1 >= len(vertices) || v2 >= len(vertices) {
			continue
		}

		baseIndex := uint32(len(meshNode.Vertices))

		for _, vi := range []int{v0, v1, v2} {
			v := vertices[vi]
			meshNode.Vertices = append(meshNode.Vertices, v)
			ext.Extend(&vec3d.T{float64(v[0]), float64(v[1]), float64(v[2])})

			if texCoords != nil && vi < len(texCoords) {
				meshNode.TexCoords = append(meshNode.TexCoords, texCoords[vi])
			} else {
				meshNode.TexCoords = append(meshNode.TexCoords, vec2.T{0, 0})
			}

			meshNode.Normals = append(meshNode.Normals, vec3.T{0, 0, 1})
		}

		faceGroup.Faces = append(faceGroup.Faces, &mst.Face{
			Vertex: [3]uint32{baseIndex, baseIndex + 1, baseIndex + 2},
		})
	}
}

func (t *TilesOsgbToMst) processDrawArrays(first int32, count int32, vertices []vec3.T, texCoords []vec2.T, meshNode *mst.MeshNode, faceGroup *mst.MeshTriangle, ext *vec3d.Box) {
	for i := first; i+2 < first+count; i += 3 {
		v0 := int(i)
		v1 := int(i + 1)
		v2 := int(i + 2)

		if v0 >= len(vertices) || v1 >= len(vertices) || v2 >= len(vertices) {
			continue
		}

		baseIndex := uint32(len(meshNode.Vertices))

		for _, vi := range []int{v0, v1, v2} {
			v := vertices[vi]
			meshNode.Vertices = append(meshNode.Vertices, v)
			ext.Extend(&vec3d.T{float64(v[0]), float64(v[1]), float64(v[2])})

			if texCoords != nil && vi < len(texCoords) {
				meshNode.TexCoords = append(meshNode.TexCoords, texCoords[vi])
			} else {
				meshNode.TexCoords = append(meshNode.TexCoords, vec2.T{0, 0})
			}

			meshNode.Normals = append(meshNode.Normals, vec3.T{0, 0, 1})
		}

		faceGroup.Faces = append(faceGroup.Faces, &mst.Face{
			Vertex: [3]uint32{baseIndex, baseIndex + 1, baseIndex + 2},
		})
	}
}

var _ FormatConvert = (*TilesOsgbToMst)(nil)
