package asset3d

import (
	"fmt"
	"path/filepath"

	"github.com/flywave/go-stl"

	mst "github.com/flywave/go-mst"
	mat4d "github.com/flywave/go3d/float64/mat4"
	vec3d "github.com/flywave/go3d/float64/vec3"
)

// StlToMst 实现从STL到MST格式的转换
// 基于现有的FbxToMst结构，提供STL文件的转换功能
type StlToMst struct {
	baseDir string
	texId   int
}

// NewStlToMst 创建一个新的STL转换器实例
func NewStlToMst() *StlToMst {
	return &StlToMst{
		texId: 0,
	}
}

// Convert 将STL文件转换为MST网格格式
// inputFilename: STL文件路径
// 返回转换后的MST网格和边界框
func (cv *StlToMst) Convert(inputFilename string) (*mst.Mesh, *[6]float64, error) {
	mesh := mst.NewMesh()

	// 读取STL文件
	solid, err := stl.ReadFile(inputFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("读取STL文件失败: %v", err)
	}

	// 设置基础目录
	cv.baseDir = filepath.Dir(inputFilename)

	// 创建默认材质
	defaultMaterial := &mst.BaseMaterial{
		Color: [3]byte{200, 200, 200}, // 默认灰色
	}
	mesh.Materials = append(mesh.Materials, defaultMaterial)

	// 创建网格节点
	meshNode := &mst.MeshNode{}

	// 转换所有三角形
	bbox := cv.convertSolidToMeshNode(solid, meshNode)

	// 计算法线
	meshNode.ReComputeNormal()

	// 添加到网格
	mesh.Nodes = append(mesh.Nodes, meshNode)

	return mesh, bbox.Array(), nil
}

// convertSolidToMeshNode 将STL固体转换为MST网格节点
func (cv *StlToMst) convertSolidToMeshNode(solid *stl.Solid, meshNode *mst.MeshNode) *vec3d.Box {
	bbox := vec3d.MinBox

	// 为每个三角形创建面组
	faceGroup := &mst.MeshTriangle{
		Batchid: int32(0), // 使用默认材质的索引
	}

	// 处理所有三角形
	for _, triangle := range solid.Triangles {
		// 添加三个顶点
		for _, vertex := range triangle.Vertices {
			meshNode.Vertices = append(meshNode.Vertices, vertex)

			// 转换为vec3d并扩展边界框
			v3d := vec3d.T{float64(vertex[0]), float64(vertex[1]), float64(vertex[2])}
			bbox.Extend(&v3d)
		}

		// 添加面（三角形）
		baseIdx := uint32(len(meshNode.Vertices) - 3)
		faceGroup.Faces = append(faceGroup.Faces, &mst.Face{
			Vertex: [3]uint32{baseIdx, baseIdx + 1, baseIdx + 2},
		})
	}

	// 设置面组
	meshNode.FaceGroup = append(meshNode.FaceGroup, faceGroup)

	return &bbox
}

// ConvertWithScale 转换STL文件并应用缩放
func (cv *StlToMst) ConvertWithScale(inputFilename string, scale float64) (*mst.Mesh, *[6]float64, error) {
	mesh := mst.NewMesh()

	// 读取STL文件
	solid, err := stl.ReadFile(inputFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("读取STL文件失败: %v", err)
	}

	// 应用缩放
	if scale != 1.0 {
		solid.Scale(scale)
	}

	// 设置基础目录
	cv.baseDir = filepath.Dir(inputFilename)

	// 创建默认材质
	defaultMaterial := &mst.BaseMaterial{
		Color: [3]byte{200, 200, 200}, // 默认灰色
	}
	mesh.Materials = append(mesh.Materials, defaultMaterial)

	// 创建网格节点
	meshNode := &mst.MeshNode{}

	// 转换所有三角形
	bbox := cv.convertSolidToMeshNode(solid, meshNode)

	// 计算法线
	meshNode.ReComputeNormal()

	// 添加到网格
	mesh.Nodes = append(mesh.Nodes, meshNode)

	return mesh, bbox.Array(), nil
}

// ConvertWithTransform 转换STL文件并应用变换矩阵
func (cv *StlToMst) ConvertWithTransform(inputFilename string, transform *mat4d.T) (*mst.Mesh, *[6]float64, error) {
	mesh := mst.NewMesh()

	// 读取STL文件
	solid, err := stl.ReadFile(inputFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("读取STL文件失败: %v", err)
	}

	// 应用变换
	if transform != nil {
		solid.Transform(transform)
	}

	// 设置基础目录
	cv.baseDir = filepath.Dir(inputFilename)

	// 创建默认材质
	defaultMaterial := &mst.BaseMaterial{
		Color: [3]byte{200, 200, 200}, // 默认灰色
	}
	mesh.Materials = append(mesh.Materials, defaultMaterial)

	// 创建网格节点
	meshNode := &mst.MeshNode{}

	// 转换所有三角形
	bbox := cv.convertSolidToMeshNode(solid, meshNode)

	// 计算法线
	meshNode.ReComputeNormal()

	// 添加到网格
	mesh.Nodes = append(mesh.Nodes, meshNode)

	return mesh, bbox.Array(), nil
}

// ConvertFromSolid 直接从STL固体对象转换
func (cv *StlToMst) ConvertFromSolid(solid *stl.Solid) (*mst.Mesh, *[6]float64, error) {
	mesh := mst.NewMesh()

	// 创建默认材质
	defaultMaterial := &mst.BaseMaterial{
		Color: [3]byte{200, 200, 200}, // 默认灰色
	}
	mesh.Materials = append(mesh.Materials, defaultMaterial)

	// 创建网格节点
	meshNode := &mst.MeshNode{}

	// 转换所有三角形
	bbox := cv.convertSolidToMeshNode(solid, meshNode)

	// 计算法线
	meshNode.ReComputeNormal()

	// 添加到网格
	mesh.Nodes = append(mesh.Nodes, meshNode)

	return mesh, bbox.Array(), nil
}

// Ensure StlToMst implements FormatConvert interface
var _ FormatConvert = (*StlToMst)(nil)
