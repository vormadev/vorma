package constants

const (
	/////// RUNTIME ARTIFACTS
	RUNTIME_DIRNAME                = ".vorma"                     // .wavedist/static/.vorma/
	RUNTIME_SNAPSHOT_DEV_FILENAME  = "runtime_snapshot_dev.json"  // .wavedist/static/.vorma/runtime_snapshot_dev.json
	RUNTIME_SNAPSHOT_PROD_FILENAME = "runtime_snapshot_prod.json" // .wavedist/static/.vorma/runtime_snapshot_prod.json
	VITE_MANIFEST_FILENAME         = "vite_manifest.json"         // .wavedist/static/.vorma/vite_manifest.json

	/////// PUBLIC ARTIFACTS
	PUBLIC_ROUTE_MANIFEST_FILENAME = "vorma_internal_route_manifest.json" // .wavedist/static/assets/public/wave_out_vorma_internal_route_manifest_<hash>.json

	/////// GENERATED TYPESCRIPT
	GENERATED_TS_INDEX_FILENAME     = "index.ts"     // <gen_out_dir>/index.ts
	GENERATED_TS_FILEMAP_FILENAME   = "filemap.ts"   // <gen_out_dir>/filemap.ts
	GENERATED_JSON_FILEMAP_FILENAME = "filemap.json" // <gen_out_dir>/filemap.json

	/////// GENERATED GO
	GENERATED_GO_IMPORTS_FILENAME = "autoreg.go" // <gen_out_dir>/autoreg.go
)
