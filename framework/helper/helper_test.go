package helper

import (
	"testing"
)

func TestStr(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{"hello", "hello"},
		{123, "123"},
		{int64(456), "456"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
		{nil, ""},
	}

	for _, tt := range tests {
		result := Str(tt.input)
		if result != tt.expected {
			t.Errorf("Str(%v) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestInt(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected int
	}{
		{123, 123},
		{"456", 456},
		{int64(789), 789},
		{float64(100.5), 100},
		{"invalid", 0},
		{nil, 0},
	}

	for _, tt := range tests {
		result := Int(tt.input)
		if result != tt.expected {
			t.Errorf("Int(%v) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestInt64(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected int64
	}{
		{123, 123},
		{"456", 456},
		{int64(789), 789},
		{float64(100.5), 100},
		{"invalid", 0},
		{nil, 0},
	}

	for _, tt := range tests {
		result := Int64(tt.input)
		if result != tt.expected {
			t.Errorf("Int64(%v) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestMd5(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "5d41402abc4b2a76b9719d911017c592"},
		{"world", "7d793037a0760186574b0282f2f435e7"},
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
	}

	for _, tt := range tests {
		result := Md5(tt.input)
		if result != tt.expected {
			t.Errorf("Md5(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestSha256(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
		{"world", "486ea46224d1bb4fb680f34f7c9ad96a8f24ec88be73ea8e5a6c65260e9cb8a7"},
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	}

	for _, tt := range tests {
		result := Sha256(tt.input)
		if result != tt.expected {
			t.Errorf("Sha256(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestStrlen(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"hello", 5},
		{"你好", 2},
		{"", 0},
	}

	for _, tt := range tests {
		result := Strlen(tt.input)
		if result != tt.expected {
			t.Errorf("Strlen(%s) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestSubstr(t *testing.T) {
	tests := []struct {
		str      string
		start    int
		length   int
		expected string
	}{
		{"hello world", 0, 5, "hello"},
		{"hello world", 6, 5, "world"},
		{"你好世界", 0, 6, "你好"},
	}

	for _, tt := range tests {
		result := Substr(tt.str, tt.start, tt.length)
		if result != tt.expected {
			t.Errorf("Substr(%s, %d, %d) = %s, expected %s", tt.str, tt.start, tt.length, result, tt.expected)
		}
	}
}

func TestTrim(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"\t\nworld\t\n", "world"},
		{"no-space", "no-space"},
	}

	for _, tt := range tests {
		result := Trim(tt.input)
		if result != tt.expected {
			t.Errorf("Trim(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestUcfirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"Hello", "Hello"},
		{"", ""},
	}

	for _, tt := range tests {
		result := Ucfirst(tt.input)
		if result != tt.expected {
			t.Errorf("Ucfirst(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestRandomString(t *testing.T) {
	result1 := RandomString(10)
	result2 := RandomString(10)

	if len(result1) != 10 {
		t.Errorf("RandomString(10) length = %d, expected 10", len(result1))
	}

	if result1 == result2 {
		t.Error("RandomString should produce different results")
	}
}

func TestRandom(t *testing.T) {
	for i := 0; i < 100; i++ {
		result := Random(1, 10)
		if result < 1 || result > 10 {
			t.Errorf("Random(1, 10) = %d, expected between 1 and 10", result)
		}
	}
}

func TestBase64EncodeDecode(t *testing.T) {
	original := "Hello, World!"
	encoded := Base64Encode(original)

	decoded, err := Base64Decode(encoded)
	if err != nil {
		t.Fatalf("Base64Decode failed: %v", err)
	}

	if decoded != original {
		t.Errorf("Base64Decode = %s, expected %s", decoded, original)
	}
}

func TestPasswordHash(t *testing.T) {
	password := "mypassword123"
	hash, err := PasswordHash(password)
	if err != nil {
		t.Fatalf("PasswordHash failed: %v", err)
	}

	if hash == password {
		t.Error("Hash should not equal password")
	}

	if !PasswordVerify(password, hash) {
		t.Error("PasswordVerify should return true for correct password")
	}

	if PasswordVerify("wrongpassword", hash) {
		t.Error("PasswordVerify should return false for wrong password")
	}
}

func TestBool(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected bool
	}{
		{true, true},
		{false, false},
		{"true", true},
		{"false", false},
		{1, true},
		{0, false},
		{nil, false},
	}

	for _, tt := range tests {
		result := Bool(tt.input)
		if result != tt.expected {
			t.Errorf("Bool(%v) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestFloat(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{3.14, 3.14},
		{"2.5", 2.5},
		{123, 123.0},
		{"invalid", 0},
		{nil, 0},
	}

	for _, tt := range tests {
		result := Float(tt.input)
		if result != tt.expected {
			t.Errorf("Float(%v) = %f, expected %f", tt.input, result, tt.expected)
		}
	}
}

func TestExplodeImplode(t *testing.T) {
	str := "a,b,c"
	parts := Explode(",", str)

	if len(parts) != 3 {
		t.Errorf("Explode length = %d, expected 3", len(parts))
	}

	result := Implode(",", parts)
	if result != str {
		t.Errorf("Implode = %s, expected %s", result, str)
	}
}

func TestStrReplace(t *testing.T) {
	result := StrReplace("world", "Vigo", "Hello world")
	if result != "Hello Vigo" {
		t.Errorf("StrReplace = %s, expected 'Hello Vigo'", result)
	}
}

func TestHtmlentities(t *testing.T) {
	result := Htmlentities("<script>alert('xss')</script>")
	if result == "<script>alert('xss')</script>" {
		t.Error("Htmlentities should escape HTML entities")
	}
}
