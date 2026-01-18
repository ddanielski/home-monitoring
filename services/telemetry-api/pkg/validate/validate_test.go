package validate

import (
	"testing"
)

func TestIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "myapp", false},
		{"valid with numbers", "myapp123", false},
		{"valid with underscore", "my_app", false},
		{"valid with hyphen", "my-app", false},
		{"valid mixed", "MyApp_v2-test", false},
		{"empty", "", true},
		{"starts with number", "123app", true},
		{"starts with underscore", "_app", true},
		{"starts with hyphen", "-app", true},
		{"contains space", "my app", true},
		{"contains special char", "my@app", true},
		{"too long", "a" + string(make([]byte, 64)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Identifier(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Identifier(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"semver", "1.0.0", false},
		{"semver with v", "v1.2.3", false},
		{"date version", "2024.01.15", false},
		{"with hyphen", "1.0.0-beta", false},
		{"with underscore", "1_0_0", false},
		{"empty", "", true},
		{"starts with dot", ".1.0", true},
		{"too long", "v" + string(make([]byte, 32)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Version(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Version(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestUUID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid lowercase", "550e8400-e29b-41d4-a716-446655440000", false},
		{"valid uppercase", "550E8400-E29B-41D4-A716-446655440000", false},
		{"valid mixed", "550e8400-E29B-41d4-A716-446655440000", false},
		{"empty", "", true},
		{"too short", "550e8400-e29b-41d4-a716", true},
		{"no hyphens", "550e8400e29b41d4a716446655440000", true},
		{"invalid chars", "550e8400-e29b-41d4-a716-44665544000g", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UUID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("UUID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestPathSegment(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "myfile", false},
		{"valid with numbers", "file123", false},
		{"empty", "", true},
		{"contains slash", "path/file", true},
		{"contains dotdot", "path..file", true},
		{"starts with dot", ".hidden", true},
		{"starts with hyphen", "-file", true},
		{"too long", string(make([]byte, 65)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PathSegment(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("PathSegment(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCommandType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid reboot", "reboot", false},
		{"valid config", "config_update", false},
		{"contains exec", "execCommand", true},
		{"contains shell", "shellScript", true},
		{"contains eval", "evalCode", true},
		{"contains system", "systemCall", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CommandType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CommandType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestTelemetryType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid temperature", "temperature", false},
		{"valid with underscore", "cpu_usage", false},
		{"empty", "", true},
		{"starts with number", "123temp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TelemetryType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TelemetryType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestStringLength(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		min     int
		max     int
		wantErr bool
	}{
		{"valid", "hello", 1, 10, false},
		{"exact min", "a", 1, 10, false},
		{"exact max", "1234567890", 1, 10, false},
		{"too short", "", 1, 10, true},
		{"too long", "12345678901", 1, 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := StringLength(tt.input, tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("StringLength(%q, %d, %d) error = %v, wantErr %v", tt.input, tt.min, tt.max, err, tt.wantErr)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := NewValidationError("field", "must be valid")
	if err.Field != "field" {
		t.Errorf("expected field 'field', got %q", err.Field)
	}
	if err.Message != "must be valid" {
		t.Errorf("expected message 'must be valid', got %q", err.Message)
	}
	expected := "field: must be valid"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}
