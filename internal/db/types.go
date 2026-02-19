package db

// IsTextType reports whether a MySQL data type is a text/string type
// that can be used as a source for text-based transforms.
func IsTextType(dataType string) bool {
	switch dataType {
	case "text", "mediumtext", "longtext", "varchar", "char":
		return true
	}
	return false
}

// IsJSONSourceType reports whether a MySQL data type is a native JSON type.
// Kept separate from IsTextType because json is not a text type semantically,
// but it still maps to text_to_jsonb when the target is jsonb.
func IsJSONSourceType(dataType string) bool {
	return dataType == "json"
}

// IsJSONType reports whether a PostgreSQL data type is a JSON/JSONB type.
func IsJSONType(dataType string) bool {
	switch dataType {
	case "json", "jsonb":
		return true
	}
	return false
}

// IsIntType reports whether a MySQL data type is an integer type.
func IsIntType(dataType string) bool {
	switch dataType {
	case "tinyint", "smallint", "mediumint", "int", "bigint":
		return true
	}
	return false
}

// IsBoolType reports whether a PostgreSQL data type is a boolean type.
func IsBoolType(dataType string) bool {
	switch dataType {
	case "boolean", "bool":
		return true
	}
	return false
}

// IsUUIDType reports whether a PostgreSQL data type is a UUID type.
func IsUUIDType(dataType string) bool {
	return dataType == "uuid"
}

// DetectTransform returns the appropriate transform name for a source→target
// column type pair, or an empty string if no transform is needed.
// Source types are MySQL data types; target types are PostgreSQL data types.
func DetectTransform(srcType, tgtType string) string {
	if (IsTextType(srcType) || IsJSONSourceType(srcType)) && IsJSONType(tgtType) {
		return "text_to_jsonb"
	}
	if IsIntType(srcType) && IsBoolType(tgtType) {
		return "int_to_bool"
	}
	if IsTextType(srcType) && IsUUIDType(tgtType) {
		return "string_to_uuid"
	}
	return ""
}
