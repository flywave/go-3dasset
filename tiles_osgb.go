package asset3d

import (
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	mst "github.com/flywave/go-mst"
	osg "github.com/flywave/go-osg"
	"github.com/flywave/go-osg/model"
	pj "github.com/flywave/go-proj"
	vec3d "github.com/flywave/go3d/float64/vec3"
	"github.com/flywave/go3d/vec2"
	"github.com/flywave/go3d/vec3"
)

var _ FormatConvert = (*TilesOsgbToMst)(nil)

type TilesOsgbToMst struct {
	currentPath   string
	dataPath      string
	origin        [3]float64
	srs           string
	loadedFiles   map[string]bool
	ApplyOrigin   bool
	GeoRef        *mst.GeoRef
	GenerateNormals bool
	texIdCounter  int32
	texDataCache  map[string]*mst.Texture
}

func (t *TilesOsgbToMst) ConvertMultiple(path string) ([]*mst.Mesh, *[6]float64, error) {
	t.currentPath = path
	t.loadedFiles = make(map[string]bool)
	t.texDataCache = make(map[string]*mst.Texture)

	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}

	// Single .osgb file → extract its finest geometry directly
	if !info.IsDir() {
		if !strings.HasSuffix(strings.ToLower(path), ".osgb") {
			return nil, nil, fmt.Errorf("unsupported file format: %s", path)
		}
		var meshes []*mst.Mesh
		ext := vec3d.MinBox
		mesh := mst.NewMesh()
		t.loadFile(path, mesh, &ext)
		if len(mesh.Nodes) > 0 {
			meshes = append(meshes, mesh)
		}
		return meshes, ext.Array(), nil
	}

	// Directory input
	metadataPath := filepath.Join(path, "metadata.xml")
	t.parseMetadata(metadataPath)

	var meshes []*mst.Mesh
	ext := vec3d.MinBox

	// Strategy 1: Data/Tile_+xxx_+xxx/ structure (ContextCapture standard)
	dataDir := filepath.Join(path, "Data")
	if s, err := os.Stat(dataDir); err == nil && s.IsDir() {
		tileDirs, _ := filepath.Glob(filepath.Join(dataDir, "Tile_*"))
		for _, tileDir := range tileDirs {
			osgbFiles, _ := filepath.Glob(filepath.Join(tileDir, "*.osgb"))
			if len(osgbFiles) == 0 {
				continue
			}
			mesh := mst.NewMesh()
			// Load only the finest LOD sub-tiles; skip the base tile to avoid
			// duplicating overlapping coarse geometry at lower LOD levels.
			finest := findFinestLod(osgbFiles)
			for _, f := range finest {
				t.loadFile(f, mesh, &ext)
			}
			if len(mesh.Nodes) > 0 {
				if geoRef := t.computeGeoRef(); geoRef != nil {
					mesh.GeoRef = geoRef
				}
				meshes = append(meshes, mesh)
			}
		}
		if len(meshes) > 0 {
			return meshes, ext.Array(), nil
		}
	}

	// Strategy 2: OSGB files directly in root directory
	osgbFiles, _ := filepath.Glob(filepath.Join(path, "*.osgb"))
	if len(osgbFiles) > 0 {
		groups := make(map[string][]string)
		for _, f := range osgbFiles {
			region := stripLODSuffix(filepath.Base(f))
			groups[region] = append(groups[region], f)
		}
		for _, files := range groups {
			if len(files) == 0 {
				continue
			}
			finest := findFinestLod(files)
			if len(finest) == 0 {
				continue
			}
			mesh := mst.NewMesh()
			for _, f := range finest {
				t.loadFile(f, mesh, &ext)
			}
			if len(mesh.Nodes) > 0 {
				if geoRef := t.computeGeoRef(); geoRef != nil {
					mesh.GeoRef = geoRef
				}
				meshes = append(meshes, mesh)
			}
		}
		return meshes, ext.Array(), nil
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

// --- file discovery ---

func findFinestLod(files []string) []string {
	if len(files) == 0 {
		return nil
	}
	bestLod := -1
	for _, f := range files {
		if l := extractLodLevel(filepath.Base(f)); l > bestLod {
			bestLod = l
		}
	}
	var result []string
	for _, f := range files {
		if extractLodLevel(filepath.Base(f)) == bestLod {
			result = append(result, f)
		}
	}
	if len(result) == 0 {
		return files[:1]
	}
	return result
}

func findFinestLodSingle(files []string) string {
	r := findFinestLod(files)
	if len(r) == 0 {
		return ""
	}
	return r[0]
}

func extractLodLevel(filename string) int {
	lod := 0
	for i := 0; i < len(filename); i++ {
		if filename[i] == 'L' && i+1 < len(filename) {
			for j := i + 1; j < len(filename) && filename[j] >= '0' && filename[j] <= '9'; j++ {
				lod = lod*10 + int(filename[j]-'0')
			}
			return lod
		}
	}
	return 0
}

func stripLODSuffix(filename string) string {
	for i := 0; i < len(filename); i++ {
		if filename[i] == 'L' && i+1 < len(filename) && filename[i+1] >= '0' && filename[i+1] <= '9' {
			// Check _L pattern
			if i > 0 && filename[i-1] == '_' {
				return filename[:i-1] + ".osgb"
			}
		}
	}
	return filename
}

// --- metadata ---

func (t *TilesOsgbToMst) parseMetadata(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var meta ModelMetadata
	if err := xml.Unmarshal(data, &meta); err != nil {
		return err
	}
	t.srs = meta.SRS
	if parts := strings.Split(meta.SRSOrigin, ","); len(parts) >= 3 {
		t.origin[0] = parseFloat(parts[0])
		t.origin[1] = parseFloat(parts[1])
		t.origin[2] = parseFloat(parts[2])
	}
	return nil
}

func (t *TilesOsgbToMst) computeGeoRef() *mst.GeoRef {
	if t.GeoRef != nil {
		return t.GeoRef
	}
	if t.srs == "" || t.srs == "unknown" {
		return nil
	}
	proj, err := pj.NewProj(t.srs)
	if err != nil {
		return nil
	}
	defer proj.Close()

	wgs84, err := pj.NewProj("+proj=longlat +datum=WGS84 +no_defs")
	if err != nil {
		return nil
	}
	defer wgs84.Close()

	lon, lat, err := pj.Transform2(proj, wgs84, t.origin[0], t.origin[1])
	if err != nil {
		return nil
	}
	ecefX, ecefY, ecefZ, err := pj.Lonlat2Ecef(lon, lat, t.origin[2])
	if err != nil {
		return nil
	}
	return &mst.GeoRef{
		EcefOrigin:   [3]float64{ecefX, ecefY, ecefZ},
		LatLonOrigin: [3]float64{lat, lon, t.origin[2]},
	}
}

// --- loading ---

func (t *TilesOsgbToMst) loadFile(osgbPath string, mesh *mst.Mesh, ext *vec3d.Box) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[ERROR] Recovered from panic: %v\n", r)
		}
	}()

	absPath, _ := filepath.Abs(osgbPath)
	if t.loadedFiles[absPath] {
		return
	}
	t.loadedFiles[absPath] = true

	rw := osg.NewReadWrite()
	res := rw.ReadNode(osgbPath, nil)
	if res == nil || res.GetNode() == nil {
		return
	}
	t.traverse(res.GetNode(), mesh, ext, filepath.Dir(osgbPath), nil, nil)
}

func (t *TilesOsgbToMst) traverse(n interface{}, mesh *mst.Mesh, ext *vec3d.Box, baseDir string, matrix *[4][4]float32, parentStates *model.StateSet) {
	// Determine effective StateSet: node's own StateSet overrides parent's
	effectiveStates := parentStates
	if nodeWithStates, ok := n.(model.NodeInterface); ok {
		if ss := nodeWithStates.GetStates(); ss != nil {
			effectiveStates = ss
		}
	}

	switch v := n.(type) {
	case *model.Geode:
		for _, c := range v.GetChildren() {
			if g, ok := c.(*model.Geometry); ok {
				t.processGeometry(g, mesh, ext, matrix, effectiveStates)
			}
		}

	case *model.Group:
		for _, c := range v.GetChildren() {
			t.traverse(c, mesh, ext, baseDir, matrix, effectiveStates)
		}

	case *model.MatrixTransform:
		combined := combineMatrix(matrix, &v.Matrix)
		for _, c := range v.GetChildren() {
			t.traverse(c, mesh, ext, baseDir, combined, effectiveStates)
		}

	case *model.PositionAttitudeTransform:
		combined := combineMatrix(matrix, patToMatrix(v.Position, v.Attitude, v.Scale))
		for _, c := range v.GetChildren() {
			t.traverse(c, mesh, ext, baseDir, combined, effectiveStates)
		}

	case *model.PagedLod:
		// Only process inline geometry (finest LOD level).
		// PagedLod file references point to lower-LOD child tiles;
		// skip them to extract only the highest-detail geometry.
		for _, c := range v.GetChildren() {
			t.traverse(c, mesh, ext, baseDir, matrix, effectiveStates)
		}
	}
}

// --- geometry processing ---

func (t *TilesOsgbToMst) processGeometry(g *model.Geometry, mesh *mst.Mesh, ext *vec3d.Box, matrix *[4][4]float32, parentStates *model.StateSet) {
	if g.VertexArray == nil || g.VertexArray.Data == nil {
		return
	}
	positions := extractVec3s(g.VertexArray, t.ApplyOrigin, t.origin)
	if len(positions) == 0 {
		return
	}
	if matrix != nil {
		positions = applyMatrix(positions, matrix)
	}

	var uvs []vec2.T
	if len(g.TexCoordArrayList) > 0 && g.TexCoordArrayList[0] != nil && g.TexCoordArrayList[0].Data != nil {
		uvs = extractVec2s(g.TexCoordArrayList[0])
	}
	hasUV := len(uvs) > 0

	meshNode := &mst.MeshNode{}
	faceGroup := &mst.MeshTriangle{Batchid: int32(len(mesh.Materials))}

	for _, prim := range g.Primitives {
		t.processPrim(prim, positions, uvs, hasUV, meshNode, faceGroup, ext)
	}

	if len(faceGroup.Faces) == 0 {
		return
	}

	if t.GenerateNormals && len(meshNode.Vertices) > 0 && len(meshNode.Normals) == 0 {
		meshNode.Normals = computeNormals(meshNode.Vertices, faceGroup.Faces)
	}
	if len(meshNode.Normals) == 0 && len(meshNode.Vertices) > 0 {
		for range meshNode.Vertices {
			meshNode.Normals = append(meshNode.Normals, vec3.T{0, 0, 1})
		}
	}

	meshNode.FaceGroup = append(meshNode.FaceGroup, faceGroup)

	mesh.Nodes = append(mesh.Nodes, meshNode)

	// Try to extract texture from StateSet (Geometry's own or inherited from parent)
	// Only apply texture if the geometry has UV coordinates; without UVs the texture
	// would sample pixel (0,0) of the atlas (often black padding).
	texture := t.extractOsgTexture(g, parentStates)
	if texture != nil && hasUV {
		texMatIdx := len(mesh.Materials)
		mesh.Materials = append(mesh.Materials, &mst.TextureMaterial{
			BaseMaterial: mst.BaseMaterial{
				Color: [3]byte{255, 255, 255},
			},
			Texture: texture,
		})
		faceGroup.Batchid = int32(texMatIdx)
	} else {
		mesh.Materials = append(mesh.Materials, &mst.BaseMaterial{Color: [3]byte{200, 200, 200}})
	}
}

func (t *TilesOsgbToMst) extractOsgTexture(g *model.Geometry, parentStates *model.StateSet) *mst.Texture {
	// Check Geometry's own StateSet first, then inherited parent StateSet
	ss := g.GetStates()
	if ss == nil {
		ss = parentStates
	}
	if ss == nil {
		return nil
	}
	for _, attrList := range ss.TextureAttributeList {
		for _, pair := range attrList {
			if pair == nil {
				continue
			}
			tex, ok := pair.First.(*model.Texture)
			if !ok || tex.Image == nil || tex.Image.Data == nil {
				continue
			}
			img := tex.Image
			w, h := int(img.S), int(img.T)
			if w <= 0 || h <= 0 {
				continue
			}
			data := img.Data
			rgbaSize := w * h * 4
			rgbSize := w * h * 3
			var rgba []byte
			if len(data) == rgbaSize {
				rgba = osgRawToRGBA(data, w, h, 4)
			} else if len(data) == rgbSize {
				rgba = osgRawToRGBA(data, w, h, 3)
			} else {
				continue
			}
			// Deduplicate textures with identical image data: cache by raw data checksum
			key := fmt.Sprintf("%dx%d_%d", w, h, xxhash(len(rgba), rgba))
			if cached, ok := t.texDataCache[key]; ok {
				return cached
			}
			t.texIdCounter++
			texObj := &mst.Texture{
				Id:         t.texIdCounter,
				Format:     mst.TEXTURE_FORMAT_RGBA,
				Size:       [2]uint64{uint64(w), uint64(h)},
				Compressed: mst.TEXTURE_COMPRESSED_ZLIB,
				Data:       mst.CompressImage(rgba),
			}
			t.texDataCache[key] = texObj
			return texObj
		}
	}
	return nil
}

func xxhash(seed int, data []byte) int {
	h := seed
	for _, b := range data {
		h = h*31 + int(b)
	}
	return h
}

func osgRawToRGBA(data []byte, w, h, bpp int) []byte {
	out := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := (y*w + x) * bpp
			di := (y*w + x) * 4
			out[di+0] = data[si+0]
			out[di+1] = data[si+1]
			out[di+2] = data[si+2]
			if bpp == 4 {
				out[di+3] = data[si+3]
			} else {
				out[di+3] = 255
			}
		}
	}
	return out
}

func getPrimMode(prim interface{}) int32 {
	switch p := prim.(type) {
	case *model.DrawElementsUInt:
		return p.Mode
	case *model.DrawElementsUShort:
		return p.Mode
	case *model.DrawElementsUByte:
		return p.Mode
	case *model.DrawArrays:
		return p.Mode
	}
	return 0
}

func (t *TilesOsgbToMst) processPrim(prim interface{}, positions []vec3.T, uvs []vec2.T, hasUV bool, node *mst.MeshNode, fg *mst.MeshTriangle, ext *vec3d.Box) {
	getIndices := func() []uint32 {
		switch p := prim.(type) {
		case *model.DrawElementsUInt:
			return p.Data
		case *model.DrawElementsUShort:
			if p.Data == nil {
				return nil
			}
			out := make([]uint32, len(p.Data))
			for i, v := range p.Data {
				out[i] = uint32(v)
			}
			return out
		case *model.DrawElementsUByte:
			if p.Data == nil {
				return nil
			}
			out := make([]uint32, len(p.Data))
			for i, v := range p.Data {
				out[i] = uint32(v)
			}
			return out
		case *model.DrawArrays:
			if p.Count < 3 {
				return nil
			}
			out := make([]uint32, p.Count)
			for i := uint32(0); i < uint32(p.Count); i++ {
				out[i] = uint32(p.First) + i
			}
			return out
		}
		return nil
	}

	indices := getIndices()
	if indices == nil || len(indices) < 3 {
		return
	}

	n := len(indices)
	mode := getPrimMode(prim)
	switch mode {
	case model.TRIANGLES:
		for i := 0; i+2 < n; i += 3 {
			t.emitTriangle(int(indices[i]), int(indices[i+1]), int(indices[i+2]), positions, uvs, hasUV, node, fg, ext)
		}
	case model.QUADS:
		for i := 0; i+3 < n; i += 4 {
			a, b, c, d := int(indices[i]), int(indices[i+1]), int(indices[i+2]), int(indices[i+3])
			t.emitTriangle(a, b, c, positions, uvs, hasUV, node, fg, ext)
			t.emitTriangle(a, c, d, positions, uvs, hasUV, node, fg, ext)
		}
	case model.TRIANGLESTRIP:
		for i := 0; i+2 < n; i++ {
			if i%2 == 0 {
				t.emitTriangle(int(indices[i]), int(indices[i+1]), int(indices[i+2]), positions, uvs, hasUV, node, fg, ext)
			} else {
				t.emitTriangle(int(indices[i+1]), int(indices[i]), int(indices[i+2]), positions, uvs, hasUV, node, fg, ext)
			}
		}
	case model.TRIANGLEFAN:
		// Triangle fan: 0-1-2, 0-2-3, 0-3-4, ...
		for i := 1; i+1 < n; i++ {
			t.emitTriangle(int(indices[0]), int(indices[i]), int(indices[i+1]), positions, uvs, hasUV, node, fg, ext)
		}
	case model.POLYGON:
		// Simple polygon triangulation: 0-1-2, 0-2-3, 0-3-4, ...
		for i := 1; i+1 < n; i++ {
			t.emitTriangle(int(indices[0]), int(indices[i]), int(indices[i+1]), positions, uvs, hasUV, node, fg, ext)
		}
	default:
		// Fallback: infer from count
		if n%3 == 0 {
			for i := 0; i+2 < n; i += 3 {
				t.emitTriangle(int(indices[i]), int(indices[i+1]), int(indices[i+2]), positions, uvs, hasUV, node, fg, ext)
			}
		} else if n%4 == 0 {
			for i := 0; i+3 < n; i += 4 {
				a, b, c, d := int(indices[i]), int(indices[i+1]), int(indices[i+2]), int(indices[i+3])
				t.emitTriangle(a, b, c, positions, uvs, hasUV, node, fg, ext)
				t.emitTriangle(a, c, d, positions, uvs, hasUV, node, fg, ext)
			}
		} else {
			for i := 0; i+2 < n; i++ {
				if i%2 == 0 {
					t.emitTriangle(int(indices[i]), int(indices[i+1]), int(indices[i+2]), positions, uvs, hasUV, node, fg, ext)
				} else {
					t.emitTriangle(int(indices[i+1]), int(indices[i]), int(indices[i+2]), positions, uvs, hasUV, node, fg, ext)
				}
			}
		}
	}
}

func (t *TilesOsgbToMst) emitTriangle(i0, i1, i2 int, positions []vec3.T, uvs []vec2.T, hasUV bool, node *mst.MeshNode, fg *mst.MeshTriangle, ext *vec3d.Box) {
	if i0 >= len(positions) || i1 >= len(positions) || i2 >= len(positions) {
		return
	}
	base := uint32(len(node.Vertices))
	for _, vi := range []int{i0, i1, i2} {
		v := positions[vi]
		node.Vertices = append(node.Vertices, v)
		ext.Extend(&vec3d.T{float64(v[0]), float64(v[1]), float64(v[2])})
		if hasUV && vi < len(uvs) {
			node.TexCoords = append(node.TexCoords, uvs[vi])
		} else {
			node.TexCoords = append(node.TexCoords, vec2.T{0, 0})
		}
	}
	fg.Faces = append(fg.Faces, &mst.Face{Vertex: [3]uint32{base, base + 1, base + 2}})
}

// --- vertex extraction ---

func extractVec3s(arr *model.Array, applyOrigin bool, origin [3]float64) []vec3.T {
	if arr == nil || arr.Data == nil {
		return nil
	}
	switch data := arr.Data.(type) {
	case [][3]float32:
		out := make([]vec3.T, len(data))
		for i, v := range data {
			if applyOrigin {
				out[i] = vec3.T{v[0] + float32(origin[0]), v[1] + float32(origin[1]), v[2] + float32(origin[2])}
			} else {
				out[i] = vec3.T{v[0], v[1], v[2]}
			}
		}
		return out
	case []float32:
		n := len(data) / 3
		out := make([]vec3.T, n)
		for i := 0; i < n; i++ {
			if applyOrigin {
				out[i] = vec3.T{data[i*3] + float32(origin[0]), data[i*3+1] + float32(origin[1]), data[i*3+2] + float32(origin[2])}
			} else {
				out[i] = vec3.T{data[i*3], data[i*3+1], data[i*3+2]}
			}
		}
		return out
	case []int16:
		n := len(data) / 3
		out := make([]vec3.T, n)
		for i := 0; i < n; i++ {
			x, y, z := float64(data[i*3])/1000, float64(data[i*3+1])/1000, float64(data[i*3+2])/1000
			if applyOrigin {
				x += origin[0]; y += origin[1]; z += origin[2]
			}
			out[i] = vec3.T{float32(x), float32(y), float32(z)}
		}
		return out
	case []uint16:
		n := len(data) / 3
		out := make([]vec3.T, n)
		for i := 0; i < n; i++ {
			x, y, z := float64(data[i*3])/1000, float64(data[i*3+1])/1000, float64(data[i*3+2])/1000
			if applyOrigin {
				x += origin[0]; y += origin[1]; z += origin[2]
			}
			out[i] = vec3.T{float32(x), float32(y), float32(z)}
		}
		return out
	case []int32:
		n := len(data) / 3
		out := make([]vec3.T, n)
		for i := 0; i < n; i++ {
			x, y, z := float64(data[i*3])/1000, float64(data[i*3+1])/1000, float64(data[i*3+2])/1000
			if applyOrigin {
				x += origin[0]; y += origin[1]; z += origin[2]
			}
			out[i] = vec3.T{float32(x), float32(y), float32(z)}
		}
		return out
	}
	return nil
}

func extractVec2s(arr *model.Array) []vec2.T {
	if arr == nil || arr.Data == nil {
		return nil
	}
	switch data := arr.Data.(type) {
	case [][2]float32:
		out := make([]vec2.T, len(data))
		for i, v := range data {
			out[i] = vec2.T{v[0], v[1]}
		}
		return out
	case []float32:
		n := len(data) / 2
		out := make([]vec2.T, n)
		for i := 0; i < n; i++ {
			out[i] = vec2.T{data[i*2], data[i*2+1]}
		}
		return out
	case []int16:
		n := len(data) / 2
		out := make([]vec2.T, n)
		for i := 0; i < n; i++ {
			out[i] = vec2.T{float32(data[i*2]), float32(data[i*2+1])}
		}
		return out
	case []int32:
		n := len(data) / 2
		out := make([]vec2.T, n)
		for i := 0; i < n; i++ {
			out[i] = vec2.T{float32(data[i*2]), float32(data[i*2+1])}
		}
		return out
	}
	return nil
}

// --- matrix helpers ---

func combineMatrix(parent, child *[4][4]float32) *[4][4]float32 {
	if parent == nil {
		return child
	}
	if child == nil {
		return parent
	}
	var r [4][4]float32
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				r[i][j] += parent[i][k] * child[k][j]
			}
		}
	}
	return &r
}

func patToMatrix(pos [3]float64, att [4]float64, scl [3]float64) *[4][4]float32 {
	var r [4][4]float32
	for i := 0; i < 4; i++ {
		r[i][i] = 1
	}
	if scl[0] == 0 || scl[1] == 0 || scl[2] == 0 || scl[0] > 1e100 {
		scl = [3]float64{1, 1, 1}
	}
	x, y, z, w := att[0], att[1], att[2], att[3]
	xx, yy, zz := x*x, y*y, z*z
	xy, xz, yz := x*y, x*z, y*z
	wx, wy, wz := w*x, w*y, w*z

	r[0][0] = float32(1 - 2*(yy+zz))
	r[0][1] = float32(2 * (xy + wz))
	r[0][2] = float32(2 * (xz - wy))
	r[1][0] = float32(2 * (xy - wz))
	r[1][1] = float32(1 - 2*(xx+zz))
	r[1][2] = float32(2 * (yz + wx))
	r[2][0] = float32(2 * (xz + wy))
	r[2][1] = float32(2 * (yz - wx))
	r[2][2] = float32(1 - 2*(xx+yy))

	r[0][0] *= float32(scl[0])
	r[0][1] *= float32(scl[0])
	r[0][2] *= float32(scl[0])
	r[1][0] *= float32(scl[1])
	r[1][1] *= float32(scl[1])
	r[1][2] *= float32(scl[1])
	r[2][0] *= float32(scl[2])
	r[2][1] *= float32(scl[2])
	r[2][2] *= float32(scl[2])

	r[3][0] = float32(pos[0])
	r[3][1] = float32(pos[1])
	r[3][2] = float32(pos[2])
	return &r
}

func applyMatrix(vs []vec3.T, m *[4][4]float32) []vec3.T {
	out := make([]vec3.T, len(vs))
	for i, v := range vs {
		x, y, z := float64(v[0]), float64(v[1]), float64(v[2])
		out[i] = vec3.T{
			float32(float64(m[0][0])*x + float64(m[1][0])*y + float64(m[2][0])*z + float64(m[3][0])),
			float32(float64(m[0][1])*x + float64(m[1][1])*y + float64(m[2][1])*z + float64(m[3][1])),
			float32(float64(m[0][2])*x + float64(m[1][2])*y + float64(m[2][2])*z + float64(m[3][2])),
		}
		w := float64(m[3][0])*x + float64(m[3][1])*y + float64(m[3][2])*z + float64(m[3][3])
		if w != 0 && w != 1 {
			out[i][0] /= float32(w)
			out[i][1] /= float32(w)
			out[i][2] /= float32(w)
		}
	}
	return out
}

// --- normal generation ---

func computeNormals(verts []vec3.T, faces []*mst.Face) []vec3.T {
	n := make([]vec3.T, len(verts))
	for _, f := range faces {
		i0, i1, i2 := int(f.Vertex[0]), int(f.Vertex[1]), int(f.Vertex[2])
		if i0 >= len(verts) || i1 >= len(verts) || i2 >= len(verts) {
			continue
		}
		a := vec3.Sub(&verts[i1], &verts[i0])
		b := vec3.Sub(&verts[i2], &verts[i0])
		fn := vec3.Cross(&a, &b)
		len2 := fn[0]*fn[0] + fn[1]*fn[1] + fn[2]*fn[2]
		if len2 < 1e-10 {
			continue
		}
		inv := 1.0 / float32(math.Sqrt(float64(len2)))
		fn[0] *= inv
		fn[1] *= inv
		fn[2] *= inv
		n[i0] = vec3.Add(&n[i0], &fn)
		n[i1] = vec3.Add(&n[i1], &fn)
		n[i2] = vec3.Add(&n[i2], &fn)
	}
	for i := range n {
		l := n[i][0]*n[i][0] + n[i][1]*n[i][1] + n[i][2]*n[i][2]
		if l < 1e-10 {
			n[i] = vec3.T{0, 0, 1}
			continue
		}
		inv := 1.0 / float32(math.Sqrt(float64(l)))
		n[i][0] *= inv
		n[i][1] *= inv
		n[i][2] *= inv
	}
	return n
}

// --- util ---
