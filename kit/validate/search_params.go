package validate

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/vormadev/vorma/kit/reflectutil"
)

// parseURLValues parses URL values into a struct.
func parseURLValues(values map[string][]string, destStructPtr any) error {
	dstValue := reflect.ValueOf(destStructPtr)
	if dstValue.Kind() != reflect.Ptr || dstValue.IsNil() {
		return fmt.Errorf("validate.parseURLValues: destination must be non-nil")
	}

	dstElem := dstValue.Elem()

	if dstElem.Kind() == reflect.Interface {
		dstElem = dstElem.Elem()
	}

	if dstElem.Kind() != reflect.Struct {
		return fmt.Errorf("validate.parseURLValues: destination must point to a struct")
	}

	return setNestedField(dstElem, values)
}

func setNestedField(v reflect.Value, values map[string][]string) error {
	t := v.Type()
	for i := range v.NumField() {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if fieldValue.Kind() == reflect.Ptr {
			kind := fieldValue.Type().Elem().Kind()
			if kind == reflect.Struct || kind == reflect.Map || kind == reflect.Slice {
				if fieldValue.IsNil() {
					fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
				}
				fieldValue = fieldValue.Elem()
			}
		}

		if !fieldValue.CanSet() {
			continue
		}

		tag := reflectutil.GetJSONFieldName(field)

		// Handle embedded structs
		if field.Anonymous {
			if err := setNestedField(fieldValue, values); err != nil {
				return err
			}
			continue
		}

		if fieldValue.Kind() == reflect.Struct {
			nestedValues := make(map[string][]string)
			prefix := tag + "."

			for key, value := range values {
				if strings.HasPrefix(key, prefix) {
					nestedValues[strings.TrimPrefix(key, prefix)] = value
				}
			}

			if err := setNestedField(fieldValue, nestedValues); err != nil {
				return err
			}

			continue
		}

		if fieldValue.Kind() == reflect.Map {
			nestedValues := make(map[string][]string)
			prefix := tag + "."

			for key, value := range values {
				if strings.HasPrefix(key, prefix) {
					nestedValues[strings.TrimPrefix(key, prefix)] = value
				}
			}

			if err := setMapField(fieldValue, nestedValues); err != nil {
				return err
			}

			continue
		}

		if fieldValue.Kind() == reflect.Slice {
			var nestedValues []string
			for key, value := range values {
				if strings.HasPrefix(key, tag) {
					// Filter out empty strings to avoid creating unintended elements
					for _, v := range value {
						if v != "" {
							nestedValues = append(nestedValues, v)
						}
					}
				}
			}
			if len(nestedValues) == 0 {
				// Set to an empty slice if no valid values are found
				fieldValue.Set(reflect.MakeSlice(fieldValue.Type(), 0, 0))
			} else if err := setSliceField(fieldValue, nestedValues); err != nil {
				return err
			}
			continue
		}

		if value, ok := values[tag]; ok {
			if err := setField(fieldValue, value); err != nil {
				return fmt.Errorf("error setting field %s: %w", field.Name, err)
			}

			continue
		}
	}

	return nil
}

func setMapField(v reflect.Value, values map[string][]string) error {
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}

	for key, value := range values {
		keyValue := reflect.ValueOf(key)
		elemValue := reflect.New(v.Type().Elem()).Elem()

		if elemValue.Kind() == reflect.Map {
			nestedValues := make(map[string][]string)
			prefix := key + "."
			for nKey, nValue := range values {
				if strings.HasPrefix(nKey, prefix) {
					nestedValues[strings.TrimPrefix(nKey, prefix)] = nValue
				}
			}
			if err := setMapField(elemValue, nestedValues); err != nil {
				return err
			}
		} else {
			if err := setField(elemValue, value); err != nil {
				return fmt.Errorf("error setting map value for key %s: %w", key, err)
			}
		}

		v.SetMapIndex(keyValue, elemValue)
	}

	return nil
}

func setField(field reflect.Value, values []string) error {
	if len(values) == 0 {
		return nil
	}

	switch field.Kind() {
	case reflect.Ptr:
		if values[0] == "" {
			// Set to nil for empty values
			field.Set(reflect.Zero(field.Type()))
			return nil
		}
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return setSingleValueField(field.Elem(), values[0])
	case reflect.Slice:
		return setSliceField(field, values)
	case reflect.Map:
		return setMapField(field, map[string][]string{"": values})
	case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool:
		return setSingleValueField(field, values[0])
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
}

func setSliceField(field reflect.Value, values []string) error {
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))
	for i, value := range values {
		elem := slice.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem.Set(reflect.New(elem.Type().Elem()))
			elem = elem.Elem()
		}
		err := setSingleValueField(elem, value)
		if err != nil {
			return err
		}
	}
	field.Set(slice)
	return nil
}

func setSingleValueField(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("field is not settable")
	}

	if value == "" {
		return nil // Do nothing for empty values on non-pointer types
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intValue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(intValue)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintValue, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(uintValue)
	case reflect.Float32, reflect.Float64:
		floatValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatValue)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolValue)
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}
