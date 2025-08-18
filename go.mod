module github.com/flywave/go-3dasset

go 1.24

toolchain go1.24.4

replace github.com/flywave/flywave-mesh => ../flywave-mesh

replace github.com/flywave/go-topo => ../go-topo

require (
	github.com/chai2010/tiff v0.0.0-20211005095045-4ec2aa243943
	github.com/flywave/gltf v0.20.4-0.20250411080706-f58af20d5f38
	github.com/flywave/go-3ds v0.0.0-20210617133319-24beedbcf9db
	github.com/flywave/go-3jsbin v0.0.0-20240203004220-1e101f10fa3e
	github.com/flywave/go-assimp v0.0.0-00010101000000-000000000000
	github.com/flywave/go-collada v0.0.0-20210617100142-f02e95c083a9
	github.com/flywave/go-mst v0.0.0-20250814104510-37f0a6660bc0
	github.com/flywave/go-obj v0.0.0-20250815235847-2e1d7495ae52
	github.com/flywave/go-stl v0.0.0-20250818070638-f2c3dee7ad76
	github.com/flywave/go3d v0.0.0-20250816053852-aed5d825659f
	github.com/flywave/ofbx v1.0.2-0.20250621073716-6719feb53699
	golang.org/x/image v0.28.0
)

require (
	github.com/oakmound/oak/v2 v2.5.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
)

replace github.com/flywave/go-geos => ../go-geos

replace github.com/flywave/go-geoid => ../go-geoid

replace github.com/flywave/go-assimp => ../go-assimp
