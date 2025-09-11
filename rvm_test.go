package asset3d

import (
	"testing"
)

func TestRvmToMst(t *testing.T) {
	// 创建RVM到MST转换器
	converter := NewRvmToMst()

	// 检查是否正确实现了FormatConvert接口
	var _ FormatConvert = converter

	// 测试带选项的转换器创建
	options := &RvmToMstOptions{
		CenterModel:       true,
		RotateZToY:        true,
		IncludeAttributes: true,
		MergeGeometries:   true,
		Anchors:           true,
	}
	converterWithOptions := NewRvmToMstWithOptions(options)
	
	// 检查是否正确实现了FormatConvert接口
	var _ FormatConvert = converterWithOptions

	// 验证工厂函数
	rvmConverter := FormatFactory(RVM)
	if rvmConverter == nil {
		t.Error("RVM格式应该在工厂函数中返回有效的转换器")
	}
	
	// 检查返回的转换器是否正确
	if _, ok := rvmConverter.(*RvmToMst); !ok {
		t.Error("工厂函数应该返回RvmToMst类型的转换器")
	}

	t.Log("RVM到MST转换器测试通过")
}