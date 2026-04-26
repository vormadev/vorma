package vormarun

// Reserved prefixes / namespacing:
// - `X-Vorma-`
// - `__VORMA_`
// - `vorma_`
// - `__vorma_`
// - `vorma-`

/////// Query Params

const Query_Key_Vorma_JSON = "vorma-json"

/////// Headers

const X_Vorma_Client_Build_Id = "X-Vorma-Client-Build-Id"
const X_Vorma_Build_Skew = "X-Vorma-Build-Skew"

/////// Filenames

const Main_CSS_Filename = "vorma_internal_main_css.css"
const Public_Static_Out_Name_Prefix = "vorma_out_"
const Prehashed_Dirname = "__prehashed"
const Prod_Tmp_Vite_Manifest_Filename = "vorma_internal_tmp_vite_manifest.json"

/////// Elements

const Main_CSS_El_ID = "vorma-main-css"
const Critical_CSS_EL_ID = "vorma-critical-css"
const Vorma_Root_El_ID = "vorma-root"
const Vorma_Data_JSON_Script_El_ID = "vorma-data-json"

/////// Attributes

const CSS_BUNDLE_ATTR = "data-vorma-css-bundle"

const (
	Meta_Start_Comment = "<!-- vorma-meta-start -->"
	Meta_End_Comment   = "<!-- vorma-meta-end -->"
	Rest_Start_Comment = "<!-- vorma-rest-start -->"
	Rest_End_Comment   = "<!-- vorma-rest-end -->"
)

/////// Router Settings

const Dynamic_Param_Prefix = ':'
const Splat_Segment_Identifier = '*'
const Explicit_Index_Segment_Identifier = "_index"

/////// Env Keys

const Env_Key_Is_Dev = "__VORMA_IS_DEV"
