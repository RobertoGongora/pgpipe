package db

import "testing"

func TestIsTextType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dataType string
		expected bool
	}{
		{"text", true},
		{"mediumtext", true},
		{"longtext", true},
		{"varchar", true},
		{"char", true},
		// Not text types
		{"int", false},
		{"bigint", false},
		{"json", false},
		{"jsonb", false},
		{"uuid", false},
		{"boolean", false},
		{"tinyint", false},
		{"", false},
		{"TEXT", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			result := IsTextType(tt.dataType)
			if result != tt.expected {
				t.Errorf("IsTextType(%q) = %v, want %v", tt.dataType, result, tt.expected)
			}
		})
	}
}

func TestIsJSONSourceType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dataType string
		expected bool
	}{
		{"json", true},
		// Not MySQL json source types
		{"jsonb", false},
		{"text", false},
		{"varchar", false},
		{"longtext", false},
		{"JSON", false}, // case-sensitive
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			result := IsJSONSourceType(tt.dataType)
			if result != tt.expected {
				t.Errorf("IsJSONSourceType(%q) = %v, want %v", tt.dataType, result, tt.expected)
			}
		})
	}
}

func TestIsJSONType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dataType string
		expected bool
	}{
		{"json", true},
		{"jsonb", true},
		// Not JSON types
		{"text", false},
		{"varchar", false},
		{"boolean", false},
		{"uuid", false},
		{"", false},
		{"JSONB", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			result := IsJSONType(tt.dataType)
			if result != tt.expected {
				t.Errorf("IsJSONType(%q) = %v, want %v", tt.dataType, result, tt.expected)
			}
		})
	}
}

func TestIsIntType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dataType string
		expected bool
	}{
		{"tinyint", true},
		{"smallint", true},
		{"mediumint", true},
		{"int", true},
		{"bigint", true},
		// Not int types
		{"boolean", false},
		{"varchar", false},
		{"text", false},
		{"float", false},
		{"double", false},
		{"decimal", false},
		{"", false},
		{"INT", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			result := IsIntType(tt.dataType)
			if result != tt.expected {
				t.Errorf("IsIntType(%q) = %v, want %v", tt.dataType, result, tt.expected)
			}
		})
	}
}

func TestIsBoolType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dataType string
		expected bool
	}{
		{"boolean", true},
		{"bool", true},
		// Not bool types
		{"tinyint", false},
		{"int", false},
		{"varchar", false},
		{"text", false},
		{"", false},
		{"BOOLEAN", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			result := IsBoolType(tt.dataType)
			if result != tt.expected {
				t.Errorf("IsBoolType(%q) = %v, want %v", tt.dataType, result, tt.expected)
			}
		})
	}
}

func TestIsUUIDType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dataType string
		expected bool
	}{
		{"uuid", true},
		// Not UUID types
		{"varchar", false},
		{"char", false},
		{"text", false},
		{"UUID", false}, // case-sensitive
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.dataType, func(t *testing.T) {
			result := IsUUIDType(tt.dataType)
			if result != tt.expected {
				t.Errorf("IsUUIDType(%q) = %v, want %v", tt.dataType, result, tt.expected)
			}
		})
	}
}

func TestDetectTransform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		srcType  string
		tgtType  string
		expected string
	}{
		// text_to_jsonb: text family → json/jsonb
		{"text to jsonb", "text", "jsonb", "text_to_jsonb"},
		{"text to json", "text", "json", "text_to_jsonb"},
		{"varchar to jsonb", "varchar", "jsonb", "text_to_jsonb"},
		{"char to jsonb", "char", "jsonb", "text_to_jsonb"},
		{"mediumtext to jsonb", "mediumtext", "jsonb", "text_to_jsonb"},
		{"longtext to jsonb", "longtext", "jsonb", "text_to_jsonb"},
		{"mediumtext to json", "mediumtext", "json", "text_to_jsonb"},
		// text_to_jsonb: MySQL native json → jsonb
		{"mysql json to jsonb", "json", "jsonb", "text_to_jsonb"},
		{"mysql json to json", "json", "json", "text_to_jsonb"},
		// int_to_bool: integer family → boolean
		{"tinyint to boolean", "tinyint", "boolean", "int_to_bool"},
		{"tinyint to bool", "tinyint", "bool", "int_to_bool"},
		{"smallint to boolean", "smallint", "boolean", "int_to_bool"},
		{"int to boolean", "int", "boolean", "int_to_bool"},
		{"bigint to boolean", "bigint", "boolean", "int_to_bool"},
		// string_to_uuid: text family → uuid
		{"varchar to uuid", "varchar", "uuid", "string_to_uuid"},
		{"char to uuid", "char", "uuid", "string_to_uuid"},
		{"text to uuid", "text", "uuid", "string_to_uuid"},
		// passthrough: no transform needed
		{"varchar to varchar", "varchar", "varchar", ""},
		{"int to int", "int", "integer", ""},
		{"text to text", "text", "text", ""},
		{"bigint to bigint", "bigint", "bigint", ""},
		{"unknown to unknown", "blob", "bytea", ""},
		// json source should NOT match uuid or bool
		{"mysql json to uuid", "json", "uuid", ""},
		{"mysql json to boolean", "json", "boolean", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectTransform(tt.srcType, tt.tgtType)
			if result != tt.expected {
				t.Errorf("DetectTransform(%q, %q) = %q, want %q",
					tt.srcType, tt.tgtType, result, tt.expected)
			}
		})
	}
}
