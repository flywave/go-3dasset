package asset3d

import (
	"os"
	"testing"

	mst "github.com/flywave/go-mst"
	"github.com/flywave/go-stl"
	"github.com/flywave/go3d/vec3"
)

func TestStlToMst_Convert(t *testing.T) {
	// 创建一个测试用的STL文件
	testSolid := &stl.Solid{
		Name: "TestCube",
		Triangles: []stl.Triangle{
			{
				Normal: vec3.T{0, 0, 1},
				Vertices: [3]vec3.T{
					{0, 0, 0}, {1, 0, 0}, {0, 1, 0},
				},
			},
			{
				Normal: vec3.T{0, 0, 1},
				Vertices: [3]vec3.T{
					{1, 0, 0}, {1, 1, 0}, {0, 1, 0},
				},
			},
		},
	}

	// 保存到临时文件
	tempFile := "test_cube.stl"
	defer os.Remove(tempFile)

	err := testSolid.WriteFile(tempFile)
	if err != nil {
		t.Fatalf("无法创建测试STL文件: %v", err)
	}

	// 测试转换
	converter := NewStlToMst()
	mesh, bbox, err := converter.Convert(tempFile)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	// 验证结果
	if mesh == nil {
		t.Fatal("返回的网格为nil")
	}

	// 验证材质
	if len(mesh.Materials) != 1 {
		t.Errorf("期望1个材质，实际%d个", len(mesh.Materials))
	} else {
		material := mesh.Materials[0]
		if baseMaterial, ok := material.(*mst.BaseMaterial); ok {
			expectedColor := [3]byte{200, 200, 200}
			if baseMaterial.Color != expectedColor {
				t.Errorf("材质颜色 = %v, 期望 %v", baseMaterial.Color, expectedColor)
			}
		} else {
			t.Errorf("材质类型不是BaseMaterial")
		}
	}

	if len(mesh.Nodes) != 1 {
		t.Errorf("期望1个节点，实际%d个", len(mesh.Nodes))
	}

	node := mesh.Nodes[0]
	if len(node.Vertices) != 6 {
		t.Errorf("期望6个顶点，实际%d个", len(node.Vertices))
	}

	if len(node.FaceGroup) != 1 {
		t.Errorf("期望1个面组，实际%d个", len(node.FaceGroup))
	}

	faceGroup := node.FaceGroup[0]
	if len(faceGroup.Faces) != 2 {
		t.Errorf("期望2个面，实际%d个", len(faceGroup.Faces))
	}

	// 验证面组使用正确的材质索引
	if faceGroup.Batchid != 0 {
		t.Errorf("面组Batchid = %d, 期望 0", faceGroup.Batchid)
	}

	// 验证边界框
	expectedMin := [6]float64{0, 0, 0, 1, 1, 0}
	for i, v := range bbox {
		if v != expectedMin[i] {
			t.Errorf("边界框[%d] = %f, 期望 %f", i, v, expectedMin[i])
		}
	}
}

func TestStlToMst_ConvertWithScale(t *testing.T) {
	// 创建一个测试用的STL文件
	testSolid := &stl.Solid{
		Name: "TestCube",
		Triangles: []stl.Triangle{
			{
				Normal: vec3.T{0, 0, 1},
				Vertices: [3]vec3.T{
					{0, 0, 0}, {1, 0, 0}, {0, 1, 0},
				},
			},
		},
	}

	// 保存到临时文件
	tempFile := "test_scale.stl"
	defer os.Remove(tempFile)

	err := testSolid.WriteFile(tempFile)
	if err != nil {
		t.Fatalf("无法创建测试STL文件: %v", err)
	}

	// 测试带缩放的转换
	converter := NewStlToMst()
	mesh, bbox, err := converter.ConvertWithScale(tempFile, 2.0)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	// 验证材质
	if len(mesh.Materials) != 1 {
		t.Errorf("期望1个材质，实际%d个", len(mesh.Materials))
	} else {
		material := mesh.Materials[0]
		if baseMaterial, ok := material.(*mst.BaseMaterial); ok {
			expectedColor := [3]byte{200, 200, 200}
			if baseMaterial.Color != expectedColor {
				t.Errorf("材质颜色 = %v, 期望 %v", baseMaterial.Color, expectedColor)
			}
		} else {
			t.Errorf("材质类型不是BaseMaterial")
		}
	}

	// 验证缩放后的边界框
	expectedMax := [6]float64{0, 0, 0, 2, 2, 0}
	for i, v := range bbox {
		if v != expectedMax[i] {
			t.Errorf("边界框[%d] = %f, 期望 %f", i, v, expectedMax[i])
		}
	}
}

func TestStlToMst_ConvertFromSolid(t *testing.T) {
	// 直接测试从Solid对象转换
	testSolid := &stl.Solid{
		Triangles: []stl.Triangle{
			{
				Vertices: [3]vec3.T{
					{1, 2, 3}, {4, 5, 6}, {7, 8, 9},
				},
			},
		},
	}

	converter := NewStlToMst()
	mesh, _, err := converter.ConvertFromSolid(testSolid)
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	if len(mesh.Nodes) != 1 {
		t.Errorf("期望1个节点，实际%d个", len(mesh.Nodes))
	}

	node := mesh.Nodes[0]
	if len(node.Vertices) != 3 {
		t.Errorf("期望3个顶点，实际%d个", len(node.Vertices))
	}

	// 验证材质
	if len(mesh.Materials) != 1 {
		t.Errorf("期望1个材质，实际%d个", len(mesh.Materials))
	} else {
		material := mesh.Materials[0]
		if baseMaterial, ok := material.(*mst.BaseMaterial); ok {
			expectedColor := [3]byte{200, 200, 200}
			if baseMaterial.Color != expectedColor {
				t.Errorf("材质颜色 = %v, 期望 %v", baseMaterial.Color, expectedColor)
			}
		} else {
			t.Errorf("材质类型不是BaseMaterial")
		}
	}

	// 验证顶点数据
	expectedVertices := []vec3.T{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
	for i, expected := range expectedVertices {
		actual := node.Vertices[i]
		if actual != expected {
			t.Errorf("顶点[%d] = %v, 期望 %v", i, actual, expected)
		}
	}
}
