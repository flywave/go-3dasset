package asset3d

import (
	"path/filepath"
	"strings"

	"github.com/flywave/go-assimp"

	mst "github.com/flywave/go-mst"
	vec3d "github.com/flywave/go3d/float64/vec3"
)

// AssimpToMst implements FormatConvert interface for ASSIMP format support
type AssimpToMst struct {
	currentPath string
}

// NewAssimpToMst creates a new AssimpToMst converter
func NewAssimpToMst() *AssimpToMst {
	return &AssimpToMst{}
}

// Convert converts an ASSIMP-supported file to MST format
func (a *AssimpToMst) Convert(path string) (*mst.Mesh, *[6]float64, error) {
	a.currentPath = path

	// Default post-processing flags for optimal conversion
	postProcessFlags := assimp.PostProcessTriangulate |
		assimp.PostProcessGenSmoothNormals |
		assimp.PostProcessCalcTangentSpace |
		assimp.PostProcessJoinIdenticalVertices |
		assimp.PostProcessOptimizeMeshes |
		assimp.PostProcessOptimizeGraph |
		assimp.PostProcessRemoveRedundantMaterials

	// Convert to MST using the existing converter
	return a.convertWithFlags(path, assimp.PostProcess(postProcessFlags))
}

// ConvertWithFlags converts an ASSIMP-supported file with custom post-processing flags
func (a *AssimpToMst) convertWithFlags(path string, postProcessFlags assimp.PostProcess) (*mst.Mesh, *[6]float64, error) {
	a.currentPath = path

	// Import file using ASSIMP
	scene, release, err := assimp.ImportFile(path, postProcessFlags)
	if err != nil {
		return nil, nil, err
	}
	defer release()

	// Convert to MST using the existing converter
	mstMesh := assimp.AssimpToMSTConverter(scene)
	if mstMesh == nil {
		return nil, nil, nil
	}

	// Calculate bounding box
	bbox := vec3d.MinBox
	for _, node := range mstMesh.Nodes {
		if node != nil {
			nodeBox := node.GetBoundbox()
			if nodeBox != nil {
				min := vec3d.T{nodeBox[0], nodeBox[1], nodeBox[2]}
				max := vec3d.T{nodeBox[3], nodeBox[4], nodeBox[5]}
				nodeBBox := vec3d.Box{Min: min, Max: max}
				bbox.Join(&nodeBBox)
			}
		}
	}

	return mstMesh, bbox.Array(), nil
}

// GetSupportedFormats returns the list of formats supported by ASSIMP
func (a *AssimpToMst) GetSupportedFormats() []string {
	return []string{
		".3ds", ".ase", ".obj", ".fbx", ".dae", ".blend", ".3mf",
		".ply", ".stl", ".x", ".dxf", ".lwo", ".lws", ".md5",
		".md3", ".md2", ".nff", ".raw", ".ac", ".ms3d", ".cob",
		".q3o", ".q3s", ".pk3", ".mdc", ".mdl", ".hmp", ".ter",
		".mdl", ".ase", ".ifc", ".step", ".iges", ".3d", ".b3d",
		".q3d", ".smd", ".vta", ".m3", ".3d", ".mdl", ".md2",
		".md3", ".mdc", ".md5", ".smd", ".vta", ".obj", ".ply",
		".stl", ".3ds", ".ase", ".lwo", ".lws", ".x", ".ac",
		".ms3d", ".cob", ".q3o", ".q3s", ".pk3", ".mdc", ".mdl",
		".hmp", ".ter", ".mdl", ".ase", ".ifc", ".step", ".iges",
	}
}

// IsFormatSupported checks if a given file extension is supported
func (a *AssimpToMst) IsFormatSupported(ext string) bool {
	ext = strings.ToLower(ext)
	supported := a.GetSupportedFormats()
	for _, format := range supported {
		if format == ext {
			return true
		}
	}
	return false
}

// GetFileExtension returns the file extension from a path
func (a *AssimpToMst) GetFileExtension(path string) string {
	return filepath.Ext(path)
}

// Ensure AssimpToMst implements FormatConvert interface
var _ FormatConvert = (*AssimpToMst)(nil)
