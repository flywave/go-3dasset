package asset3d

import (
	"os"
	"path/filepath"
	"strings"

	mst "github.com/flywave/go-mst"
	gobj "github.com/flywave/go-obj"
	vec3d "github.com/flywave/go3d/float64/vec3"

	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
)

type ObjToMst struct {
	currentPath string
}

func (obj *ObjToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	obj.currentPath = path
	ext := vec3d.MinBox
	reader := &gobj.ObjReader{}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	err = reader.Read(file)
	if err != nil {
		return nil, nil, err
	}

	mesh := mst.NewMesh()
	meshNode := &mst.MeshNode{}

	// Group faces by material
	materialGroups := make(map[string]*mst.MeshTriangle)
	materialIndexMap := make(map[string]int)
	materialCounter := 0

	// Process all faces
	for _, face := range reader.F {
		materialName := face.Material
		if materialName == "" {
			materialName = "default"
		}

		// Get or create material group
		mtg, exists := materialGroups[materialName]
		if !exists {
			mtg = &mst.MeshTriangle{Batchid: int32(materialCounter)}
			materialGroups[materialName] = mtg
			materialIndexMap[materialName] = materialCounter
			materialCounter++
		}

		// Process face vertices
		if len(face.Corners) >= 3 {
			// Triangulate face
			triangles := obj.triangulateFace(face)

			// Process each triangle
			for _, triangle := range triangles {
				obj.processTriangle(mtg, triangle, reader, meshNode, &ext)
			}
		}
	}

	// Collect all non-empty material groups
	var faceGroups []*mst.MeshTriangle
	for _, group := range materialGroups {
		if len(group.Faces) > 0 {
			faceGroups = append(faceGroups, group)
		}
	}
	meshNode.FaceGroup = faceGroups

	// Create materials
	materials := obj.createMaterials(reader, materialIndexMap)
	mesh.Materials = materials

	mesh.Nodes = append(mesh.Nodes, meshNode)

	return mesh, ext.Array(), nil
}

func (obj *ObjToMst) createMaterials(reader *gobj.ObjReader, materialIndexMap map[string]int) []mst.MeshMaterial {
	materials := make([]mst.MeshMaterial, len(materialIndexMap))

	// If no materials, create a default one
	if len(materialIndexMap) == 0 {
		return []mst.MeshMaterial{&mst.BaseMaterial{
			Color: [3]byte{255, 255, 255},
		}}
	}

	// Load materials from MTL file if available
	var objMaterials map[string]*gobj.Material
	if reader.MTL != "" {
		mtlPath := reader.MTL
		if !strings.HasPrefix(mtlPath, "/") {
			// Try to find MTL file in same directory as OBJ
			objDir := filepath.Dir(obj.currentPath)
			mtlPath = filepath.Join(objDir, reader.MTL)
		}

		loadedMaterials, err := gobj.ReadMaterials(mtlPath)
		if err == nil {
			objMaterials = loadedMaterials
		}
	}

	for name, index := range materialIndexMap {
		var material mst.MeshMaterial

		// Get corresponding go-obj material
		var objMat *gobj.Material
		if objMaterials != nil {
			objMat = objMaterials[name]
		}

		if objMat == nil {
			// Create default BaseMaterial if no MTL data
			material = &mst.BaseMaterial{
				Color:        [3]byte{200, 200, 200},
				Transparency: 1.0,
			}
		} else {
			// Convert based on illumination model and material properties
			material = obj.convertMaterial(objMat)
		}

		materials[index] = material
	}

	return materials
}

func (obj *ObjToMst) convertMaterial(objMat *gobj.Material) mst.MeshMaterial {
	// Determine material type based on properties
	hasTexture := objMat.DiffuseTexture != "" || objMat.AmbientTexture != "" ||
		objMat.SpecularTexture != "" || objMat.EmissiveTexture != ""

	// Convert colors from float32[3] to byte[3]
	diffuseColor := obj.float32ToByteColor(objMat.Diffuse)
	ambientColor := obj.float32ToByteColor(objMat.Ambient)
	specularColor := obj.float32ToByteColor(objMat.Specular)
	emissiveColor := obj.float32ToByteColor(objMat.Emissive)

	// Check if this is a PBR material (has metallic/roughness properties)
	if objMat.Metallic > 0 || objMat.Roughness > 0 {
		pbrMat := &mst.PbrMaterial{
			TextureMaterial: mst.TextureMaterial{
				BaseMaterial: mst.BaseMaterial{
					Color:        diffuseColor,
					Transparency: float32(objMat.Opacity),
				},
			},
			Emissive:            emissiveColor,
			Metallic:            objMat.Metallic,
			Roughness:           objMat.Roughness,
			Reflectance:         0.5,
			AmbientOcclusion:    1.0,
			ClearCoat:           objMat.ClearcoatThickness,
			ClearCoatRoughness:  objMat.ClearcoatRoughness,
			Anisotropy:          objMat.Anisotropy,
			AnisotropyDirection: vec3.T{1, 0, 0},
			SheenColor:          [3]byte{128, 128, 128},
			SubSurfaceColor:     [3]byte{128, 128, 128},
		}

		// Load textures using convertTex
		if objMat.DiffuseTexture != "" {
			if tex := obj.loadTexture(objMat.DiffuseTexture); tex != nil {
				pbrMat.Texture = tex
			}
		}
		if objMat.BumpTexture != "" {
			if tex := obj.loadTexture(objMat.BumpTexture); tex != nil {
				pbrMat.Normal = tex
			}
		}
		return pbrMat
	}

	// Check if this is a Phong material (has specular properties)
	if objMat.Shininess > 0 || (objMat.Specular[0] > 0 || objMat.Specular[1] > 0 || objMat.Specular[2] > 0) {
		phongMat := &mst.PhongMaterial{
			LambertMaterial: mst.LambertMaterial{
				TextureMaterial: mst.TextureMaterial{
					BaseMaterial: mst.BaseMaterial{
						Color:        diffuseColor,
						Transparency: float32(objMat.Opacity),
					},
				},
				Ambient:  ambientColor,
				Diffuse:  diffuseColor,
				Emissive: emissiveColor,
			},
			Specular:    specularColor,
			Shininess:   float32(objMat.Shininess * 100), // Convert from OBJ range to typical shininess
			Specularity: 1.0,
		}

		// Load textures using convertTex
		if objMat.DiffuseTexture != "" {
			if tex := obj.loadTexture(objMat.DiffuseTexture); tex != nil {
				phongMat.Texture = tex
			}
		}
		if objMat.BumpTexture != "" {
			if tex := obj.loadTexture(objMat.BumpTexture); tex != nil {
				phongMat.Normal = tex
			}
		}
		return phongMat
	}

	// Check if this is a Lambert material (basic diffuse lighting)
	if objMat.Diffuse[0] > 0 || objMat.Diffuse[1] > 0 || objMat.Diffuse[2] > 0 {
		lambertMat := &mst.LambertMaterial{
			TextureMaterial: mst.TextureMaterial{
				BaseMaterial: mst.BaseMaterial{
					Color:        diffuseColor,
					Transparency: float32(objMat.Opacity),
				},
			},
			Ambient:  ambientColor,
			Diffuse:  diffuseColor,
			Emissive: emissiveColor,
		}

		// Load textures using convertTex
		if objMat.DiffuseTexture != "" {
			if tex := obj.loadTexture(objMat.DiffuseTexture); tex != nil {
				lambertMat.Texture = tex
			}
		}
		return lambertMat
	}

	// Default to TextureMaterial if only textures are provided
	if hasTexture {
		textureMat := &mst.TextureMaterial{
			BaseMaterial: mst.BaseMaterial{
				Color:        diffuseColor,
				Transparency: float32(objMat.Opacity),
			},
		}

		// Load textures using convertTex
		if objMat.DiffuseTexture != "" {
			if tex := obj.loadTexture(objMat.DiffuseTexture); tex != nil {
				textureMat.Texture = tex
			}
		}
		if objMat.BumpTexture != "" {
			if tex := obj.loadTexture(objMat.BumpTexture); tex != nil {
				textureMat.Normal = tex
			}
		}

		return textureMat
	}

	// Default to BaseMaterial
	return &mst.BaseMaterial{
		Color:        diffuseColor,
		Transparency: float32(objMat.Opacity),
	}
}

func (obj *ObjToMst) loadTexture(texturePath string) *mst.Texture {
	if texturePath == "" {
		return nil
	}

	// Resolve texture path relative to OBJ file
	objDir := filepath.Dir(obj.currentPath)
	fullPath := filepath.Join(objDir, texturePath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// Try alternative paths
		baseName := filepath.Base(texturePath)
		fullPath = filepath.Join(objDir, baseName)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return nil
		}
	}

	// Use convertTex to load and process the texture
	texture, err := convertTex(fullPath, 0)
	if err != nil {
		return nil
	}

	return texture
}

func (obj *ObjToMst) float32ToByteColor(color []float32) [3]byte {
	if len(color) < 3 {
		return [3]byte{255, 255, 255}
	}

	r := byte(color[0] * 255)
	g := byte(color[1] * 255)
	b := byte(color[2] * 255)

	return [3]byte{r, g, b}
}

func (obj *ObjToMst) triangulateFace(face gobj.Face) [][]gobj.FaceCorner {
	// Simple fan triangulation for convex polygons
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

func (obj *ObjToMst) processTriangle(mtg *mst.MeshTriangle, triangle []gobj.FaceCorner, reader *gobj.ObjReader, meshNode *mst.MeshNode, ext *vec3d.Box) {
	// Ensure we have exactly 3 vertices
	if len(triangle) != 3 {
		return
	}

	// Get vertex positions
	var positions [3]vec3.T
	var texCoords [3]vec2.T
	var normals [3]vec3.T

	for i, corner := range triangle {
		// Vertex position
		if corner.VertexIndex >= 0 && corner.VertexIndex < len(reader.V) {
			positions[i] = reader.V[corner.VertexIndex]
		} else {
			positions[i] = vec3.T{0, 0, 0}
		}

		// Texture coordinate
		if corner.TexCoordIndex >= 0 && corner.TexCoordIndex < len(reader.VT) {
			texCoords[i] = reader.VT[corner.TexCoordIndex]
		} else {
			texCoords[i] = vec2.T{0, 0}
		}

		// Normal
		if corner.NormalIndex >= 0 && corner.NormalIndex < len(reader.VN) {
			normals[i] = reader.VN[corner.NormalIndex]
		} else {
			// Calculate flat normal
			normals[i] = obj.calculateNormal(positions[0], positions[1], positions[2])
		}

		// Extend bounding box
		ext.Extend(&vec3d.T{float64(positions[i][0]), float64(positions[i][1]), float64(positions[i][2])})
	}

	// Add vertices to mesh node
	baseIndex := uint32(len(meshNode.Vertices))
	for i := 0; i < 3; i++ {
		meshNode.Vertices = append(meshNode.Vertices, vec3.T(positions[i]))
		meshNode.TexCoords = append(meshNode.TexCoords, texCoords[i])
		meshNode.Normals = append(meshNode.Normals, normals[i])
	}

	// Add face
	mtg.Faces = append(mtg.Faces, &mst.Face{
		Vertex: [3]uint32{baseIndex, baseIndex + 1, baseIndex + 2},
	})
}

func (obj *ObjToMst) calculateNormal(v0, v1, v2 vec3.T) vec3.T {
	// Calculate cross product for normal
	e1 := vec3.Sub(&v1, &v0)
	e2 := vec3.Sub(&v2, &v0)
	normal := vec3.Cross(&e1, &e2)

	// Normalize
	length := normal.Length()
	if length > 0 {
		return vec3.T{normal[0] / length, normal[1] / length, normal[2] / length}
	}
	return vec3.T{0, 1, 0} // Default normal
}

// Ensure ObjToMst implements FormatConvert interface
var _ FormatConvert = (*ObjToMst)(nil)
