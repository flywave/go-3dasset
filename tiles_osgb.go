package asset3d

import (
	"encoding/xml"
	"fmt"
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

func (t *TilesOsgbToMst) ConvertMultiple(path string) ([]*mst.Mesh, *[6]float64, error) {
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

	var meshes []*mst.Mesh
	ext := vec3d.MinBox

	mainFile := filepath.Join(t.dataPath, "main.osgb")
	if _, err := os.Stat(mainFile); err == nil {
		mesh := mst.NewMesh()
		t.processOsgbFile(mainFile, mesh, &ext)
		if len(mesh.Nodes) > 0 {
			meshes = append(meshes, mesh)
		}
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
				mesh := mst.NewMesh()
				t.processOsgbFile(finestFile, mesh, &ext)
				if len(mesh.Nodes) > 0 {
					meshes = append(meshes, mesh)
				}
			}
		}
	}

	return meshes, ext.Array(), nil
}

func (t *TilesOsgbToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	meshes, ext, err := t.ConvertMultiple(path)
	if err != nil {
		return nil, ext, err
	}
	if len(meshes) == 0 {
		return nil, ext, nil
	}
	return meshes[0], ext, nil
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
			fmt.Printf("[ERROR] Recovered from panic: %v\n", r)
		}
	}()

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
	t.traverseNodeWithMatrix(node, mesh, ext, baseDir, nil)
}

func (t *TilesOsgbToMst) traverseNodeWithMatrix(node model.NodeInterface, mesh *mst.Mesh, ext *vec3d.Box, baseDir string, parentMatrix *[4][4]float32) {
	switch n := node.(type) {
	case *model.Group:
		for _, child := range n.Children {
			t.traverseNodeWithMatrix(child, mesh, ext, baseDir, parentMatrix)
		}
	case *model.MatrixTransform:
		combinedMatrix := t.combineMatrix(parentMatrix, &n.Matrix)
		for _, child := range n.Children {
			t.traverseNodeWithMatrix(child, mesh, ext, baseDir, combinedMatrix)
		}
	case *model.PositionAttitudeTransform:
		matrix := t.positionAttitudeToMatrix(n.Position, n.Attitude, n.Scale)
		combinedMatrix := t.combineMatrix(parentMatrix, matrix)
		for _, child := range n.Children {
			t.traverseNodeWithMatrix(child, mesh, ext, baseDir, combinedMatrix)
		}
	case *model.PagedLod:
		for _, child := range n.Children {
			t.traverseNodeWithMatrix(child, mesh, ext, baseDir, parentMatrix)
		}
		for _, perData := range n.PerRangeDataList {
			if perData.FileName != "" {
				childPath := perData.FileName
				childPath = strings.ReplaceAll(childPath, "\\", "/")
				if !filepath.IsAbs(childPath) {
					childPath = filepath.Join(baseDir, childPath)
				}
				absPath, _ := filepath.Abs(childPath)
				if _, err := os.Stat(childPath); err == nil {
					if !t.loadedFiles[absPath] {
						t.loadedFiles[absPath] = true
						t.processOsgbFile(childPath, mesh, ext)
					}
				}
			}
		}
		for _, perData := range n.PerRangeDataList {
			if perData.FileName != "" {
				childPath := perData.FileName
				childPath = strings.ReplaceAll(childPath, "\\", "/")
				if !filepath.IsAbs(childPath) {
					childPath = filepath.Join(baseDir, childPath)
				}
				absPath, _ := filepath.Abs(childPath)
				if fi, err := os.Stat(childPath); err == nil {
					fmt.Printf("[DEBUG] File exists: %s, size: %d\n", childPath, fi.Size())
					if !t.loadedFiles[absPath] {
						t.loadedFiles[absPath] = true
						fmt.Printf("[DEBUG] Loading tile file: %s\n", childPath)
						t.processOsgbFile(childPath, mesh, ext)
						fmt.Printf("[DEBUG] Finished loading: %s\n", childPath)
					}
				}
			}
		}
		for _, perData := range n.PerRangeDataList {
			if perData.FileName != "" {
				childPath := perData.FileName
				childPath = strings.ReplaceAll(childPath, "\\", "/")
				if !filepath.IsAbs(childPath) {
					childPath = filepath.Join(baseDir, childPath)
				}
				absPath, _ := filepath.Abs(childPath)
				if fi, err := os.Stat(childPath); err == nil {
					fmt.Printf("[DEBUG] File exists: %s, size: %d\n", childPath, fi.Size())
					if !t.loadedFiles[absPath] {
						t.loadedFiles[absPath] = true
						t.processOsgbFile(childPath, mesh, ext)
					}
				} else {
					fmt.Printf("[DEBUG] File not found: %s, error: %v\n", childPath, err)
				}
			}
		}
	case *model.Geode:
		for _, child := range n.Children {
			if geom, ok := child.(*model.Geometry); ok {
				t.processGeometryWithMatrix(geom, mesh, ext, parentMatrix)
			}
		}
	case *model.Geometry:
		t.processGeometryWithMatrix(n, mesh, ext, parentMatrix)
	}
}

func (t *TilesOsgbToMst) combineMatrix(parent, child *[4][4]float32) *[4][4]float32 {
	if parent == nil {
		return child
	}
	if child == nil {
		return parent
	}
	result := new([4][4]float32)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			result[i][j] = 0
			for k := 0; k < 4; k++ {
				result[i][j] += parent[i][k] * child[k][j]
			}
		}
	}
	return result
}

func (t *TilesOsgbToMst) positionAttitudeToMatrix(position [3]float64, attitude [4]float64, scale [3]float64) *[4][4]float32 {
	result := new([4][4]float32)

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if i == j {
				result[i][j] = 1
			} else {
				result[i][j] = 0
			}
		}
	}

	if scale[0] == 0 || scale[1] == 0 || scale[2] == 0 ||
		scale[0] > 1e100 || scale[1] > 1e100 || scale[2] > 1e100 {
		scale = [3]float64{1, 1, 1}
	}

	x, y, z, w := attitude[0], attitude[1], attitude[2], attitude[3]
	xx, yy, zz := x*x, y*y, z*z
	xy, xz, yz := x*y, x*z, y*z
	wx, wy, wz := w*x, w*y, w*z

	result[0][0] = float32(1 - 2*(yy+zz))
	result[0][1] = float32(2 * (xy + wz))
	result[0][2] = float32(2 * (xz - wy))

	result[1][0] = float32(2 * (xy - wz))
	result[1][1] = float32(1 - 2*(xx+zz))
	result[1][2] = float32(2 * (yz + wx))

	result[2][0] = float32(2 * (xz + wy))
	result[2][1] = float32(2 * (yz - wx))
	result[2][2] = float32(1 - 2*(xx+yy))

	result[0][0] *= float32(scale[0])
	result[0][1] *= float32(scale[0])
	result[0][2] *= float32(scale[0])
	result[1][0] *= float32(scale[1])
	result[1][1] *= float32(scale[1])
	result[1][2] *= float32(scale[1])
	result[2][0] *= float32(scale[2])
	result[2][1] *= float32(scale[2])
	result[2][2] *= float32(scale[2])

	result[3][0] = float32(position[0])
	result[3][1] = float32(position[1])
	result[3][2] = float32(position[2])

	return result
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

func (t *TilesOsgbToMst) processGeometryWithMatrix(geom *model.Geometry, mesh *mst.Mesh, ext *vec3d.Box, matrix *[4][4]float32) {
	if geom.VertexArray == nil || geom.VertexArray.Data == nil {
		fmt.Printf("[DEBUG] processGeometry: no vertex array\n")
		return
	}

	vertices := t.extractVec3Array(geom.VertexArray)
	if len(vertices) == 0 {
		fmt.Printf("[DEBUG] processGeometry: no vertices extracted\n")
		return
	}

	fmt.Printf("[DEBUG] processGeometry: %d vertices\n", len(vertices))
	if len(vertices) > 0 {
		fmt.Printf("[DEBUG] First vertex: %v\n", vertices[0])
	}

	if matrix != nil {
		vertices = t.applyMatrixToVertices(vertices, matrix)
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

func (t *TilesOsgbToMst) applyMatrixToVertices(vertices []vec3.T, matrix *[4][4]float32) []vec3.T {
	result := make([]vec3.T, len(vertices))
	for i, v := range vertices {
		x := float64(v[0])
		y := float64(v[1])
		z := float64(v[2])
		w := float64(matrix[3][0])*x + float64(matrix[3][1])*y + float64(matrix[3][2])*z + float64(matrix[3][3])
		result[i] = vec3.T{
			float32(float64(matrix[0][0])*x + float64(matrix[1][0])*y + float64(matrix[2][0])*z + float64(matrix[3][0])),
			float32(float64(matrix[0][1])*x + float64(matrix[1][1])*y + float64(matrix[2][1])*z + float64(matrix[3][1])),
			float32(float64(matrix[0][2])*x + float64(matrix[1][2])*y + float64(matrix[2][2])*z + float64(matrix[3][2])),
		}
		if w != 0 && w != 1 {
			result[i][0] /= float32(w)
			result[i][1] /= float32(w)
			result[i][2] /= float32(w)
		}
	}
	return result
}

func (t *TilesOsgbToMst) extractVec3Array(arr *model.Array) []vec3.T {
	if arr == nil || arr.Data == nil {
		return nil
	}

	switch data := arr.Data.(type) {
	case []float32:
		count := len(data) / 3
		if count <= 0 {
			return nil
		}
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
	case []int16:
		count := len(data) / 3
		if count <= 0 {
			return nil
		}
		result := make([]vec3.T, count)
		for i := 0; i < count; i++ {
			x := float64(data[i*3]) / 1000.0
			y := float64(data[i*3+1]) / 1000.0
			z := float64(data[i*3+2]) / 1000.0
			if t.ApplyOrigin {
				x += t.origin[0]
				y += t.origin[1]
				z += t.origin[2]
			}
			result[i] = vec3.T{float32(x), float32(y), float32(z)}
		}
		return result
	case []int32:
		count := len(data) / 3
		if count <= 0 {
			return nil
		}
		result := make([]vec3.T, count)
		for i := 0; i < count; i++ {
			x := float64(data[i*3]) / 1000.0
			y := float64(data[i*3+1]) / 1000.0
			z := float64(data[i*3+2]) / 1000.0
			if t.ApplyOrigin {
				x += t.origin[0]
				y += t.origin[1]
				z += t.origin[2]
			}
			result[i] = vec3.T{float32(x), float32(y), float32(z)}
		}
		return result
	case []uint16:
		count := len(data) / 3
		if count <= 0 {
			return nil
		}
		result := make([]vec3.T, count)
		for i := 0; i < count; i++ {
			x := float64(data[i*3]) / 1000.0
			y := float64(data[i*3+1]) / 1000.0
			z := float64(data[i*3+2]) / 1000.0
			if t.ApplyOrigin {
				x += t.origin[0]
				y += t.origin[1]
				z += t.origin[2]
			}
			result[i] = vec3.T{float32(x), float32(y), float32(z)}
		}
		return result
	case []vec3.T:
		count := len(data)
		if count <= 0 {
			return nil
		}
		result := make([]vec3.T, count)
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
	}
	return nil
}

func (t *TilesOsgbToMst) extractVec2Array(arr *model.Array) []vec2.T {
	if arr == nil || arr.Data == nil {
		return nil
	}

	switch data := arr.Data.(type) {
	case []float32:
		count := len(data) / 2
		if count <= 0 {
			return nil
		}
		result := make([]vec2.T, count)
		for i := 0; i < count; i++ {
			result[i] = vec2.T{data[i*2], data[i*2+1]}
		}
		return result
	case []int16:
		count := len(data) / 2
		if count <= 0 {
			return nil
		}
		result := make([]vec2.T, count)
		for i := 0; i < count; i++ {
			result[i] = vec2.T{float32(data[i*2]), float32(data[i*2+1])}
		}
		return result
	case []int32:
		count := len(data) / 2
		if count <= 0 {
			return nil
		}
		result := make([]vec2.T, count)
		for i := 0; i < count; i++ {
			result[i] = vec2.T{float32(data[i*2]), float32(data[i*2+1])}
		}
		return result
	case []vec2.T:
		count := len(data)
		if count <= 0 {
			return nil
		}
		return data
	}
	return nil
}

func (t *TilesOsgbToMst) processPrimitive(prim interface{}, vertices []vec3.T, texCoords []vec2.T, meshNode *mst.MeshNode, faceGroup *mst.MeshTriangle, ext *vec3d.Box) {
	if prim == nil {
		return
	}
	switch p := prim.(type) {
	case *model.DrawElementsUShort:
		if p.Data != nil {
			t.processDrawElementsUShort(p.Data, vertices, texCoords, meshNode, faceGroup, ext)
		}
	case *model.DrawElementsUInt:
		if p.Data != nil {
			t.processDrawElementsUInt(p.Data, vertices, texCoords, meshNode, faceGroup, ext)
		}
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
