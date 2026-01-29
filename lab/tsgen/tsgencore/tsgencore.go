package tsgencore

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/reflectutil"
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC API & TYPES
/////////////////////////////////////////////////////////////////////

type IDStr = string
type _results = map[IDStr]*TypeInfo

// Results holds the final, processed output of a generation pass.
type Results struct {
	Types     []*TypeInfo
	id_to_idx map[IDStr]int
}

// AdHocType represents a Go type that needs a TypeScript definition.
type AdHocType struct {
	TypeInstance any
	TSTypeName   string
}

// TSTyper is an interface that a struct can implement to provide custom TypeScript type overrides.
type TSTyper interface {
	TSType() map[string]string
}

// ProcessTypes is the main entry point. It takes a slice of ad-hoc types and returns the complete, resolved Results.
func ProcessTypes(adHocTypes []*AdHocType) Results {
	allResults := make([]_results, 0, len(adHocTypes))
	for _, adHocType := range adHocTypes {
		result, _ := traverseType(adHocType)
		allResults = append(allResults, result)
	}
	return mergeTypeResults(allResults...)
}

// GetTypeInfo allows retrieving the information for a specific type from the final results.
func (r *Results) GetTypeInfo(adHocType *AdHocType) *TypeInfo {
	id := getID(adHocType)
	if idx, ok := r.id_to_idx[id]; ok {
		return r.Types[idx]
	}
	reflectType := getEffectiveReflectType(adHocType.TypeInstance)
	return &TypeInfo{
		_id:          id,
		OriginalName: adHocType.TSTypeName,
		ReflectType:  reflectType,
		TSStr:        getBasicTSType(reflectType),
	}
}

/////////////////////////////////////////////////////////////////////
/////// INTERNAL TYPES & STATE
/////////////////////////////////////////////////////////////////////

type TypeInfo struct {
	OriginalName   string
	ResolvedName   string
	ReflectType    reflect.Type
	TSStr          string
	_id            IDStr
	IsRoot         bool
	IsReferenced   bool
	UsedAsEmbedded bool
}

type typeCollector struct {
	types             map[reflect.Type]*typeEntry
	rootType          reflect.Type
	rootRequestedName string
}

type typeEntry struct {
	originalGoType reflect.Type
	usedAsEmbedded bool
	isReferenced   bool
	visited        bool
	coreType       string
	requestedName  string
}

func newTypeCollector() *typeCollector {
	return &typeCollector{types: make(map[reflect.Type]*typeEntry)}
}

/////////////////////////////////////////////////////////////////////
/////// CORE TRAVERSAL & GENERATION LOGIC
/////////////////////////////////////////////////////////////////////

var idRegex = regexp.MustCompile(`\$tsgen\$[^$]+\$tsgen\$`)

func traverseType(adHocType *AdHocType) (_results, IDStr) {
	if adHocType == nil || adHocType.TypeInstance == nil {
		return _results{}, ""
	}
	t := getEffectiveReflectType(adHocType.TypeInstance)
	effectiveRequestedName := getEffectiveRequestedName(t, adHocType.TSTypeName)
	c := newTypeCollector()
	c.rootType = t
	c.rootRequestedName = effectiveRequestedName
	c.collectType(t, adHocType.TSTypeName)
	return c.buildDefinitions()
}

func (c *typeCollector) buildDefinitions() (_results, IDStr) {
	for t, entry := range c.types {
		if entry.coreType == "" {
			if t != nil && t.Kind() == reflect.Struct {
				fields := c.generateStructTypeFields(t)
				entry.coreType = buildObj(fields)
			} else {
				entry.coreType = c.getTypeScriptType(t)
			}
		}
	}
	results := make(map[IDStr]*TypeInfo, len(c.types))
	for t, entry := range c.types {
		if t == nil {
			continue
		}
		id := getIDFromReflectType(t, entry.requestedName)
		results[id] = &TypeInfo{
			_id:            id,
			OriginalName:   entry.requestedName,
			ReflectType:    t,
			TSStr:          entry.coreType,
			IsRoot:         t == c.rootType,
			IsReferenced:   entry.isReferenced,
			UsedAsEmbedded: entry.usedAsEmbedded,
		}
	}
	rootIdStr := ""
	if c.rootType != nil {
		rootIdStr = getIDFromReflectType(c.rootType, c.rootRequestedName)
	}
	return results, rootIdStr
}

func mergeTypeResults(results ..._results) Results {
	if len(results) == 0 {
		return Results{}
	}
	flattened := make(map[IDStr]*TypeInfo)
	for _, result := range results {
		for id, typeInfo := range result {
			if existing, ok := flattened[id]; ok {
				existing.IsRoot = existing.IsRoot || typeInfo.IsRoot
				existing.IsReferenced = existing.IsReferenced || typeInfo.IsReferenced
				existing.UsedAsEmbedded = existing.UsedAsEmbedded || typeInfo.UsedAsEmbedded
			} else {
				flattened[id] = typeInfo
			}
		}
	}
	requiredIDs := make(map[IDStr]bool)
	for _, typeInfo := range flattened {
		matches := idRegex.FindAllString(typeInfo.TSStr, -1)
		for _, match := range matches {
			requiredIDs[match] = true
		}
	}
	finalTypeIDs := make([]IDStr, 0, len(flattened))
	nameUsageCounter := make(map[string]int)
	for id, typeInfo := range flattened {
		if typeInfo.IsRoot || typeInfo.IsReferenced || requiredIDs[id] {
			finalTypeIDs = append(finalTypeIDs, id)
			if typeInfo.OriginalName != "" {
				nameUsageCounter[typeInfo.OriginalName]++
			}
		}
	}
	slices.Sort(finalTypeIDs)
	finalTypes := make([]*TypeInfo, 0, len(finalTypeIDs))
	id_to_idx := make(map[string]int, len(finalTypeIDs))
	nameVersions := make(map[string]int)
	for _, id := range finalTypeIDs {
		typeInfo := flattened[id]
		effectiveName := typeInfo.OriginalName
		if effectiveName != "" && nameUsageCounter[effectiveName] > 1 {
			nameVersions[effectiveName]++
			version := nameVersions[effectiveName]
			if version > 1 {
				effectiveName = fmt.Sprintf("%s_%d", effectiveName, version)
			}
		}
		typeInfo.ResolvedName = effectiveName
		id_to_idx[typeInfo._id] = len(finalTypes)
		finalTypes = append(finalTypes, typeInfo)
	}
	for _, typeInfo := range finalTypes {
		typeInfo.TSStr = idRegex.ReplaceAllStringFunc(typeInfo.TSStr, func(id string) string {
			if idx, ok := id_to_idx[id]; ok {
				return finalTypes[idx].ResolvedName
			}
			fmt.Printf(
				"tsgencore warning: A reference to an unresolved type ID '%s' was found in the definition for '%s'. This may be due to using TSTyper to reference a type that was filtered out. Falling back to 'unknown'.\n",
				id,
				typeInfo.ResolvedName,
			)
			return "unknown"
		})
	}
	return Results{
		Types:     finalTypes,
		id_to_idx: id_to_idx,
	}
}

/////////////////////////////////////////////////////////////////////
/////// TYPE COLLECTOR METHODS
/////////////////////////////////////////////////////////////////////

func (c *typeCollector) getOrCreateEntry(t reflect.Type, userDefinedAlias ...string) *typeEntry {
	if entry, exists := c.types[t]; exists {
		if t == c.rootType && c.rootRequestedName != "" && entry.requestedName == "" {
			entry.requestedName = c.rootRequestedName
		}
		return entry
	}
	entry := &typeEntry{originalGoType: t}
	if t == c.rootType && c.rootRequestedName != "" {
		entry.requestedName = c.rootRequestedName
	} else if len(userDefinedAlias) > 0 && userDefinedAlias[0] != "" {
		entry.requestedName = userDefinedAlias[0]
	} else {
		if !isBasicType(t) {
			if n := toSanitizedName(t); n != "" {
				entry.requestedName = n
			}
		}
	}
	c.types[t] = entry
	return entry
}

func (c *typeCollector) collectType(t reflect.Type, userDefinedAlias ...string) {
	if t == nil {
		return
	}
	isRoot := (t == c.rootType)
	if t.Name() != "" || isRoot {
		entry := c.getOrCreateEntry(t, userDefinedAlias...)
		if entry.visited {
			return
		}
		entry.visited = true
		if !isRoot && isBasicType(t) {
			entry.coreType = c.getTypeScriptType(t)
			return
		}
	} else if !isRoot && isBasicType(t) {
		return
	}
	switch t.Kind() {
	case reflect.Struct:
		c.collectStructFields(t)
	case reflect.Ptr:
		if t.Name() != "" {
			c.getOrCreateEntry(t, userDefinedAlias...)
		}
		c.collectType(t.Elem())
	case reflect.Slice, reflect.Array:
		if t.Name() != "" {
			c.getOrCreateEntry(t, userDefinedAlias...)
		}
		c.collectType(t.Elem())
	case reflect.Map:
		if t.Name() != "" {
			c.getOrCreateEntry(t, userDefinedAlias...)
		}
		c.collectType(t.Key())
		c.collectType(t.Elem())
	}
}

func (c *typeCollector) collectStructFields(t reflect.Type) {
	for i := range t.NumField() {
		field := t.Field(i)
		if isUnexported(field) || shouldOmitField(field) {
			continue
		}
		if field.Anonymous {
			fieldType := field.Type
			isPtr := fieldType.Kind() == reflect.Ptr
			if isPtr {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				embeddedEntry := c.getOrCreateEntry(fieldType)
				embeddedEntry.usedAsEmbedded = true
				if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
					embeddedEntry.isReferenced = true
				}
				c.collectType(fieldType)
			}
			continue
		}
		c.collectFieldType(field.Type)
	}
}

func (c *typeCollector) collectFieldType(t reflect.Type) {
	switch t.Kind() {
	case reflect.Struct:
		c.getOrCreateEntry(t).isReferenced = true
		c.collectType(t)
	case reflect.Ptr:
		if t.Name() != "" {
			c.getOrCreateEntry(t).isReferenced = true
		}
		elemType := t.Elem()
		if elemType.Kind() == reflect.Struct {
			c.getOrCreateEntry(elemType).isReferenced = true
		}
		c.collectType(elemType)
	case reflect.Slice, reflect.Array:
		if t.Name() != "" {
			c.getOrCreateEntry(t).isReferenced = true
		}
		elemType := t.Elem()
		if elemType.Kind() == reflect.Struct {
			c.getOrCreateEntry(elemType).isReferenced = true
		} else if elemType.Kind() == reflect.Ptr && elemType.Elem().Kind() == reflect.Struct {
			c.getOrCreateEntry(elemType.Elem()).isReferenced = true
		}
		c.collectType(elemType)
	case reflect.Map:
		if t.Name() != "" {
			c.getOrCreateEntry(t).isReferenced = true
		}
		c.collectType(t.Key())
		c.collectType(t.Elem())
	}
}

/////////////////////////////////////////////////////////////////////
/////// TS STRING GENERATION
/////////////////////////////////////////////////////////////////////

func (c *typeCollector) generateStructTypeFields(t reflect.Type) []string {
	if t.Kind() != reflect.Struct {
		return nil
	}

	var fields []string
	tsTypeMap := getTSTypeMap(t)

	// Keep track of which TSType() overrides have been applied.
	usedOverrides := make(map[string]bool)

	// A recursive function to process fields in the correct, deterministic order.
	var processFields func(currentType reflect.Type, isEmbeddedPtr bool)
	processFields = func(currentType reflect.Type, isEmbeddedPtr bool) {
		// Note: We get the TSTypeMap of the *original* top-level struct 't',
		// not the embedded one. Overrides are not inherited this way.

		for i := range currentType.NumField() {
			field := currentType.Field(i)
			if isUnexported(field) || shouldOmitField(field) {
				continue
			}

			// First, handle untagged anonymous fields recursively to match `encoding/json` order.
			if field.Anonymous && field.Tag.Get("json") == "" {
				embeddedType := field.Type
				isPtr := embeddedType.Kind() == reflect.Ptr
				if isPtr {
					embeddedType = embeddedType.Elem()
				}
				processFields(embeddedType, isPtr || isEmbeddedPtr)
				continue
			}

			jsonFieldName := reflectutil.GetJSONFieldName(field)
			if jsonFieldName == "" {
				continue
			}

			var fieldType string

			// Precedence: TSType() method > ts_type tag > reflection.
			if customType, ok := tsTypeMap[field.Name]; ok {
				fieldType = customType
				usedOverrides[field.Name] = true // Mark this override as used.
			} else if customType := get_ts_type_from_struct_tag(field); customType != "" {
				fieldType = customType
			} else {
				fieldType = c.getTypeScriptType(field.Type)
			}

			if isEmbeddedPtr || isOptionalField(field) {
				fields = append(fields, fmt.Sprintf("%s?: %s", jsonFieldName, fieldType))
			} else {
				fields = append(fields, fmt.Sprintf("%s: %s", jsonFieldName, fieldType))
			}
		}
	}

	processFields(t, false)

	// Finally, add any fields from TSType() that did not match a field on the Go struct.
	if tsTypeMap != nil {
		additiveKeys := make([]string, 0)
		for key := range tsTypeMap {
			if !usedOverrides[key] {
				additiveKeys = append(additiveKeys, key)
			}
		}
		slices.Sort(additiveKeys) // Sort purely additive fields for determinism.

		for _, key := range additiveKeys {
			// Assume additive fields are required.
			fields = append(fields, fmt.Sprintf("%s: %s", key, tsTypeMap[key]))
		}
	}

	return fields
}

func (c *typeCollector) getTypeScriptType(t reflect.Type) string {
	if t == nil {
		return "null"
	}
	switch t.Kind() {
	case reflect.Interface:
		return "unknown"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Ptr:
		return c.getTypeScriptType(t.Elem())
	case reflect.Slice, reflect.Array:
		// Byte slices are serialized as base64 by encoding/json
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return "string"
		}
		return fmt.Sprintf("Array<%s>", c.getTypeScriptType(t.Elem()))
	case reflect.Map:
		return fmt.Sprintf("Record<%s, %s>", c.getTypeScriptType(t.Key()), c.getTypeScriptType(t.Elem()))
	case reflect.Struct:
		switch {
		case t == reflect.TypeOf(time.Time{}):
			return "string"
		case t == reflect.TypeOf(time.Duration(0)):
			return "number"
		case t.Name() != "":
			entry := c.getOrCreateEntry(t)
			return getIDFromReflectType(t, entry.requestedName)
		default:
			return buildObj(c.generateStructTypeFields(t))
		}
	default:
		return "unknown"
	}
}

func getBasicTSType(t reflect.Type) string {
	if t == nil {
		return "null"
	}
	switch t.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	default:
		return "unknown"
	}
}

func buildObj(fields []string) string {
	if len(fields) == 0 {
		return "Record<never, never>"
	}
	var sb strings.Builder
	sb.WriteString("{\n")
	for _, field := range fields {
		sb.WriteString("\t")
		sb.WriteString(field)
		sb.WriteString(";\n")
	}
	sb.WriteString("}")
	return sb.String()
}

/////////////////////////////////////////////////////////////////////
/////// REFLECTION & STRING HELPERS
/////////////////////////////////////////////////////////////////////

var _any any
var _null_id = getID(&AdHocType{TypeInstance: nil})
var _unknown_id = getID(&AdHocType{TypeInstance: &_any})

func (t *TypeInfo) IsTSNull() bool      { return t._id == _null_id }
func (t *TypeInfo) IsTSUnknown() bool   { return t._id == _unknown_id }
func (t *TypeInfo) IsTSBasicType() bool { return isBasicType(t.ReflectType) }

func getID(adHocType *AdHocType) IDStr {
	t := getEffectiveReflectType(adHocType.TypeInstance)
	return getIDFromReflectType(t, adHocType.TSTypeName)
}

func getIDFromReflectType(t reflect.Type, requestedName string) IDStr {
	natural_name := getNaturalName(t)
	effective_requested_name := getEffectiveRequestedName(t, requestedName)
	if effective_requested_name != "" && effective_requested_name != natural_name {
		return fmt.Sprintf("$tsgen$%v+%s$tsgen$", t, requestedName)
	}
	return fmt.Sprintf("$tsgen$%v$tsgen$", t)
}

func getEffectiveReflectType(instance any) reflect.Type {
	t := reflect.TypeOf(instance)
	if t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func getEffectiveRequestedName(t reflect.Type, requestedName string) string {
	if requestedName != "" {
		return requestedName
	}
	return getNaturalName(t)
}

func getNaturalName(t reflect.Type) string {
	if t != nil {
		n := toSanitizedName(t)
		if n != "" && isBasicType(t) {
			return ""
		}
		return n
	}
	return ""
}

var invalidJSIdentifierChars = regexp.MustCompile(`[^a-zA-Z0-9_$]`)

func toSanitizedName(t reflect.Type) string {
	if t == nil {
		return ""
	}
	return sanitizeTypeName(t.Name())
}

func sanitizeTypeName(name string) string {
	x := invalidJSIdentifierChars.ReplaceAllString(name, "_")
	if len(x) > 0 && x[len(x)-1] == '_' {
		x = x[:len(x)-1]
	}
	return x
}

func isBasicType(t reflect.Type) bool {
	if t == nil {
		return false
	}
	if t == reflect.TypeOf(time.Time{}) || t == reflect.TypeOf(time.Duration(0)) {
		return true
	}
	switch t.Kind() {
	case reflect.Interface, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return true
	default:
		return false
	}
}

func isUnexported(field reflect.StructField) bool {
	return field.PkgPath != ""
}

func isOptionalField(field reflect.StructField) bool {
	if field.Type.Kind() == reflect.Ptr {
		return true
	}
	tag := field.Tag.Get("json")
	if tag != "" {
		parts := strings.Split(tag, ",")
		for _, part := range parts[1:] {
			if part == "omitempty" || part == "omitzero" {
				return true
			}
		}
	}
	return false
}

func shouldOmitField(field reflect.StructField) bool {
	tag := field.Tag.Get("json")
	return tag == "-" || strings.HasPrefix(tag, "-,")
}

func get_ts_type_from_struct_tag(field reflect.StructField) string {
	return field.Tag.Get("ts_type")
}

func getTSTypeMap(t reflect.Type) map[string]string {
	if t == nil {
		return nil
	}
	// We need to check both the value and a pointer to it for the interface.
	ifaceType := reflect.TypeOf((*TSTyper)(nil)).Elem()

	// Case 1: The type itself implements the interface.
	if t.Implements(ifaceType) {
		instance := reflect.New(t).Elem() // Get a value of the type
		initializeEmbeddedPointers(instance.Addr())
		return instance.Interface().(TSTyper).TSType()
	}

	// Case 2: A pointer to the type implements the interface.
	pt := reflect.PointerTo(t)
	if pt.Implements(ifaceType) {
		instance := reflect.New(t) // Get a pointer to a new value
		initializeEmbeddedPointers(instance)
		return instance.Interface().(TSTyper).TSType()
	}

	return nil
}

func initializeEmbeddedPointers(v reflect.Value) {
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return
	}
	typ := elem.Type()
	for i := 0; i < elem.NumField(); i++ {
		field := elem.Field(i)
		fieldType := typ.Field(i)
		if fieldType.Anonymous && field.Kind() == reflect.Ptr && field.IsNil() {
			newValue := reflect.New(field.Type().Elem())
			field.Set(newValue)
			initializeEmbeddedPointers(newValue)
		}
	}
}
