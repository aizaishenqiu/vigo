package validate

import (
	"testing"
)

func TestValidatorRequired(t *testing.T) {
	v := New()
	v.Required("name", "")

	if v.IsValid() {
		t.Error("Expected validation to fail for empty value")
	}

	if v.Errors["name"] == "" {
		t.Error("Expected error for name field")
	}
}

func TestValidatorRequiredSuccess(t *testing.T) {
	v := New()
	v.Required("name", "John")

	if !v.IsValid() {
		t.Errorf("Expected validation to pass, got errors: %v", v.Errors)
	}
}

func TestValidatorEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected bool
	}{
		{"test@example.com", true},
		{"invalid-email", false},
		{"user@domain", false},
		{"", false},
	}

	for _, tt := range tests {
		v := New()
		v.Email("email", tt.email)

		if tt.expected {
			if !v.IsValid() {
				t.Errorf("Expected email %s to be valid, got error: %v", tt.email, v.Errors)
			}
		} else {
			if v.IsValid() {
				t.Errorf("Expected email %s to be invalid", tt.email)
			}
		}
	}
}

func TestValidatorMobile(t *testing.T) {
	tests := []struct {
		mobile   string
		expected bool
	}{
		{"13812345678", true},
		{"18812345678", true},
		{"12345678901", false},
		{"1381234567", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		v := New()
		v.Mobile("mobile", tt.mobile)

		if tt.expected {
			if !v.IsValid() {
				t.Errorf("Expected mobile %s to be valid, got error: %v", tt.mobile, v.Errors)
			}
		} else {
			if v.IsValid() {
				t.Errorf("Expected mobile %s to be invalid", tt.mobile)
			}
		}
	}
}

func TestValidatorMin(t *testing.T) {
	tests := []struct {
		value    string
		min      int
		expected bool
	}{
		{"hello", 3, true},
		{"hi", 3, false},
		{"abc", 3, true},
	}

	for _, tt := range tests {
		v := New()
		v.Min("value", tt.value, tt.min)

		if tt.expected {
			if !v.IsValid() {
				t.Errorf("Expected value %s to be >= %d", tt.value, tt.min)
			}
		} else {
			if v.IsValid() {
				t.Errorf("Expected value %s to be < %d", tt.value, tt.min)
			}
		}
	}
}

func TestValidatorMax(t *testing.T) {
	tests := []struct {
		value    string
		max      int
		expected bool
	}{
		{"hi", 5, true},
		{"hello world", 5, false},
		{"abc", 3, true},
	}

	for _, tt := range tests {
		v := New()
		v.Max("value", tt.value, tt.max)

		if tt.expected {
			if !v.IsValid() {
				t.Errorf("Expected value %s to be <= %d", tt.value, tt.max)
			}
		} else {
			if v.IsValid() {
				t.Errorf("Expected value %s to be > %d", tt.value, tt.max)
			}
		}
	}
}

func TestValidatorIn(t *testing.T) {
	tests := []struct {
		value    interface{}
		list     []interface{}
		expected bool
	}{
		{"apple", []interface{}{"apple", "banana", "orange"}, true},
		{"grape", []interface{}{"apple", "banana", "orange"}, false},
		{1, []interface{}{1, 2, 3}, true},
		{4, []interface{}{1, 2, 3}, false},
	}

	for _, tt := range tests {
		v := New()
		v.In("value", tt.value, tt.list)

		if tt.expected {
			if !v.IsValid() {
				t.Errorf("Expected value %v to be in list %v", tt.value, tt.list)
			}
		} else {
			if v.IsValid() {
				t.Errorf("Expected value %v to not be in list %v", tt.value, tt.list)
			}
		}
	}
}

func TestValidatorChaining(t *testing.T) {
	v := New()
	v.Required("name", "John").
		Required("email", "test@example.com").
		Email("email", "test@example.com").
		Min("password", "123456", 6)

	if !v.IsValid() {
		t.Errorf("Expected validation to pass, got errors: %v", v.Errors)
	}
}

func TestValidatorMultipleErrors(t *testing.T) {
	v := New()
	v.Required("name", "").
		Email("email", "invalid").
		Min("password", "123", 6)

	if v.IsValid() {
		t.Error("Expected validation to fail")
	}

	if len(v.Errors) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(v.Errors))
	}
}

func TestValidatorErrors(t *testing.T) {
	v := New()
	v.Required("name", "")

	errors := v.Errors
	if errors["name"] == "" {
		t.Error("Expected error message for name field")
	}
}
