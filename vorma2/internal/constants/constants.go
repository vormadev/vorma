package constants

const (
	/////// RUNTIME ARTIFACTS
	RUNTIME_DIRNAME                = ".vorma"                     // .waveout/static/.vorma/
	RUNTIME_SNAPSHOT_DEV_FILENAME  = "runtime_snapshot_dev.json"  // .waveout/static/.vorma/runtime_snapshot_dev.json
	RUNTIME_SNAPSHOT_PROD_FILENAME = "runtime_snapshot_prod.json" // .waveout/static/.vorma/runtime_snapshot_prod.json
	VITE_MANIFEST_FILENAME         = "vite_manifest.json"         // .waveout/static/.vorma/vite_manifest.json

	/////// PUBLIC ARTIFACTS
	PUBLIC_ROUTE_MANIFEST_FILENAME = "vorma_internal_route_manifest.json" // .waveout/static/assets/public/wave_out_vorma_internal_route_manifest_<hash>.json

	/////// GENERATED TYPESCRIPT
	GENERATED_TS_INDEX_FILENAME   = "index.ts"   // <ts_gen_out_dir>/index.ts
	GENERATED_TS_FILEMAP_FILENAME = "filemap.ts" // <ts_gen_out_dir>/filemap.ts

	/////// GENERATED GO
	GENERATED_GO_IMPORTS_FILENAME = "imports.gen.go" // <ts_gen_out_dir>/imports.gen.go
)
