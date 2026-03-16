package asset3d

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	mst "github.com/flywave/go-mst"
	gobj "github.com/flywave/go-obj"
	vec3d "github.com/flywave/go3d/float64/vec3"

	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
)

type TilesObjToMst struct {
	currentPath string
	origin      [3]float64
	ApplyOrigin bool
}

type ModelMetadata struct {
	SRS       string `xml:"SRS"`
	SRSOrigin string `xml:"SRSOrigin"`
}

func (t *TilesObjToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	t.currentPath = path

	metadataPath := filepath.Join(path, "metadata.xml")
	if err := t.parseMetadata(metadataPath); err != nil {
		return nil, nil, err
	}

	dataDir := filepath.Join(path, "Data")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return nil, nil, err
	}

	mesh := mst.NewMesh()
	ext := vec3d.MinBox

	tileDirs, err := filepath.Glob(filepath.Join(dataDir, "Tile_*"))
	if err != nil {
		return nil, nil, err
	}

	for _, tileDir := range tileDirs {
		objFiles, err := filepath.Glob(filepath.Join(tileDir, "*.obj"))
		if err != nil {
			continue
		}
		for _, objFile := range objFiles {
			if err := t.processObjFile(objFile, mesh, &ext); err != nil {
				continue
			}
		}
	}

	return mesh, ext.Array(), nil
}

func ReadTileOrigin(path string) (vec3d.T, error) {
	metadataPath := filepath.Join(path, "metadata.xml")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return vec3d.T{}, err
	}

	var metadata ModelMetadata
	if err := xml.Unmarshal(data, &metadata); err != nil {
		return vec3d.T{}, err
	}

	origin := vec3d.T{}
	parts := strings.Split(metadata.SRSOrigin, ",")
	if len(parts) >= 3 {
		origin[0] = parseFloat(parts[0])
		origin[1] = parseFloat(parts[1])
		origin[2] = parseFloat(parts[2])
	}

	return origin, nil
}

func (t *TilesObjToMst) parseMetadata(path string) error {
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

func ConvertToGlb(mesh *mst.Mesh, path string) error {
	doc, err := mst.MstToGltf([]*mst.Mesh{mesh})
	if err != nil {
		return err
	}

	data, err := mst.GetGltfBinary(doc, 4)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func (t *TilesObjToMst) processObjFile(objPath string, mesh *mst.Mesh, ext *vec3d.Box) error {
	reader := &gobj.ObjReader{}

	file, err := os.Open(objPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := reader.Read(file); err != nil {
		return err
	}

	meshNode := &mst.MeshNode{}

	materialGroups := make(map[string]*mst.MeshTriangle)
	materialIndexMap := make(map[string]int)
	baseMaterialIndex := len(mesh.Materials)
	localMaterialCounter := 0

	for _, face := range reader.F {
		materialName := face.Material
		if materialName == "" {
			materialName = "default"
		}

		mtg, exists := materialGroups[materialName]
		if !exists {
			mtg = &mst.MeshTriangle{Batchid: int32(baseMaterialIndex + localMaterialCounter)}
			materialGroups[materialName] = mtg
			materialIndexMap[materialName] = localMaterialCounter
			localMaterialCounter++
		}

		if len(face.Corners) >= 3 {
			triangles := t.triangulateFace(face)
			for _, triangle := range triangles {
				t.processTriangle(mtg, triangle, reader, meshNode, ext, objPath)
			}
		}
	}

	var faceGroups []*mst.MeshTriangle
	for _, group := range materialGroups {
		if len(group.Faces) > 0 {
			faceGroups = append(faceGroups, group)
		}
	}
	meshNode.FaceGroup = faceGroups

	materials := t.createMaterials(reader, materialIndexMap, objPath)
	mesh.Materials = append(mesh.Materials, materials...)

	if len(meshNode.FaceGroup) > 0 {
		mesh.Nodes = append(mesh.Nodes, meshNode)
	}

	return nil
}

func (t *TilesObjToMst) createMaterials(reader *gobj.ObjReader, materialIndexMap map[string]int, objPath string) []mst.MeshMaterial {
	if len(materialIndexMap) == 0 {
		return []mst.MeshMaterial{&mst.BaseMaterial{
			Color: [3]byte{255, 255, 255},
		}}
	}

	var objMaterials map[string]*gobj.Material
	if reader.MTL != "" {
		mtlPath := reader.MTL
		if !filepath.IsAbs(mtlPath) {
			mtlPath = filepath.Join(filepath.Dir(objPath), reader.MTL)
		}

		loadedMaterials, err := gobj.ReadMaterials(mtlPath)
		if err == nil {
			objMaterials = loadedMaterials
		}
	}

	materials := make([]mst.MeshMaterial, len(materialIndexMap))
	for name, index := range materialIndexMap {
		var material mst.MeshMaterial

		var objMat *gobj.Material
		if objMaterials != nil {
			objMat = objMaterials[name]
		}

		if objMat == nil {
			material = &mst.BaseMaterial{
				Color:        [3]byte{200, 200, 200},
				Transparency: 0,
			}
		} else {
			material = t.convertMaterial(objMat, objPath)
		}

		materials[index] = material
	}

	return materials
}

func (t *TilesObjToMst) convertMaterial(objMat *gobj.Material, objPath string) mst.MeshMaterial {
	diffuseColor := t.float32ToByteColor(objMat.Diffuse)

	textureMat := &mst.TextureMaterial{
		BaseMaterial: mst.BaseMaterial{
			Color:        diffuseColor,
			Transparency: 1 - float32(objMat.Opacity),
		},
	}

	if objMat.DiffuseTexture != "" {
		if tex := t.loadTexture(objMat.DiffuseTexture, objPath); tex != nil {
			textureMat.Texture = tex
		}
	}

	return textureMat
}

func (t *TilesObjToMst) loadTexture(texturePath string, objPath string) *mst.Texture {
	if texturePath == "" {
		return nil
	}

	objDir := filepath.Dir(objPath)
	fullPath := filepath.Join(objDir, texturePath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		baseName := filepath.Base(texturePath)
		fullPath = filepath.Join(objDir, baseName)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return nil
		}
	}

	texture, err := convertTex(fullPath, 0)
	if err != nil {
		return nil
	}

	return texture
}

func (t *TilesObjToMst) float32ToByteColor(color []float32) [3]byte {
	if len(color) < 3 {
		return [3]byte{255, 255, 255}
	}

	r := byte(color[0] * 255)
	g := byte(color[1] * 255)
	b := byte(color[2] * 255)

	return [3]byte{r, g, b}
}

func (t *TilesObjToMst) triangulateFace(face gobj.Face) [][]gobj.FaceCorner {
	if len(face.Corners) == 3 {
		return [][]gobj.FaceCorner{face.Corners}
	}

	var triangles [][]gobj.FaceCorner
	for i := 1; i < len(face.Corners)-1; i++ {
		triangle := []gobj.FaceCorner{
			face.Corners[0],
			face.Corners[i],
			face.Corners[i+1],
		}
		triangles = append(triangles, triangle)
	}
	return triangles
}

func (t *TilesObjToMst) processTriangle(mtg *mst.MeshTriangle, triangle []gobj.FaceCorner, reader *gobj.ObjReader, meshNode *mst.MeshNode, ext *vec3d.Box, objPath string) {
	if len(triangle) != 3 {
		return
	}

	var positions [3]vec3.T
	var texCoords [3]vec2.T
	var normals [3]vec3.T

	for i, corner := range triangle {
		if corner.VertexIndex >= 0 && corner.VertexIndex < len(reader.V) {
			v := reader.V[corner.VertexIndex]
			if t.ApplyOrigin {
				positions[i] = vec3.T{
					float32(float64(v[0]) + t.origin[0]),
					float32(float64(v[1]) + t.origin[1]),
					float32(float64(v[2]) + t.origin[2]),
				}
			} else {
				positions[i] = vec3.T{
					float32(v[0]),
					float32(v[1]),
					float32(v[2]),
				}
			}
		} else {
			positions[i] = vec3.T{0, 0, 0}
		}

		if corner.TexCoordIndex >= 0 && corner.TexCoordIndex < len(reader.VT) {
			texCoords[i] = reader.VT[corner.TexCoordIndex]
		} else {
			texCoords[i] = vec2.T{0, 0}
		}

		if corner.NormalIndex >= 0 && corner.NormalIndex < len(reader.VN) {
			normals[i] = reader.VN[corner.NormalIndex]
		} else {
			normals[i] = t.calculateNormal(positions[0], positions[1], positions[2])
		}

		ext.Extend(&vec3d.T{float64(positions[i][0]), float64(positions[i][1]), float64(positions[i][2])})
	}

	baseIndex := uint32(len(meshNode.Vertices))
	for i := 0; i < 3; i++ {
		meshNode.Vertices = append(meshNode.Vertices, vec3.T(positions[i]))
		meshNode.TexCoords = append(meshNode.TexCoords, texCoords[i])
		meshNode.Normals = append(meshNode.Normals, normals[i])
	}

	mtg.Faces = append(mtg.Faces, &mst.Face{
		Vertex: [3]uint32{baseIndex, baseIndex + 1, baseIndex + 2},
	})
}

func (t *TilesObjToMst) calculateNormal(v0, v1, v2 vec3.T) vec3.T {
	e1 := vec3.Sub(&v1, &v0)
	e2 := vec3.Sub(&v2, &v0)
	normal := vec3.Cross(&e1, &e2)

	length := normal.Length()
	if length > 0 {
		return vec3.T{normal[0] / length, normal[1] / length, normal[2] / length}
	}
	return vec3.T{0, 1, 0}
}

var _ FormatConvert = (*TilesObjToMst)(nil)
