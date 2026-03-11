package mvc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)

	if c == nil {
		t.Error("Expected context to be created")
	}

	if c.Request == nil {
		t.Error("Expected request to be set")
	}

	if c.Writer == nil {
		t.Error("Expected writer to be set")
	}
}

func TestContextQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?name=John&age=25", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)

	if c.Query("name") != "John" {
		t.Errorf("Expected name 'John', got '%s'", c.Query("name"))
	}

	if c.Query("age") != "25" {
		t.Errorf("Expected age '25', got '%s'", c.Query("age"))
	}

	if c.Query("nonexistent") != "" {
		t.Error("Expected empty string for nonexistent query param")
	}
}

func TestContextQueryDefault(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)

	if c.QueryDefault("name", "default") != "default" {
		t.Error("Expected default value for nonexistent query param")
	}
}

func TestContextInput(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?name=John", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)

	if c.Input("name") != "John" {
		t.Errorf("Expected name 'John', got '%s'", c.Input("name"))
	}
}

func TestContextInputInt(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?age=25", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)

	if c.InputInt("age", 0) != 25 {
		t.Errorf("Expected age 25, got %d", c.InputInt("age", 0))
	}

	if c.InputInt("nonexistent", 10) != 10 {
		t.Error("Expected default value for nonexistent param")
	}
}

func TestContextInputBool(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"on", true},
		{"yes", true},
		{"0", false},
		{"false", false},
		{"", false},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/test?flag="+tt.value, nil)
		w := httptest.NewRecorder()
		c := NewContext(w, req)

		if c.InputBool("flag") != tt.expected {
			t.Errorf("InputBool(%s) = %v, expected %v", tt.value, c.InputBool("flag"), tt.expected)
		}
	}
}

func TestContextGetHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer token123")
	w := httptest.NewRecorder()

	c := NewContext(w, req)

	if c.GetHeader("Authorization") != "Bearer token123" {
		t.Errorf("Expected header 'Bearer token123', got '%s'", c.GetHeader("Authorization"))
	}
}

func TestContextSetHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)
	c.SetHeader("X-Custom", "value")

	if w.Header().Get("X-Custom") != "value" {
		t.Errorf("Expected header 'value', got '%s'", w.Header().Get("X-Custom"))
	}
}

func TestContextCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})
	w := httptest.NewRecorder()

	c := NewContext(w, req)

	cookie, err := c.Cookie("session")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cookie != "abc123" {
		t.Errorf("Expected cookie 'abc123', got '%s'", cookie)
	}
}

func TestContextString(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)
	c.String(200, "Hello, World!")

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "Hello, World!" {
		t.Errorf("Expected body 'Hello, World!', got '%s'", w.Body.String())
	}
}

func TestContextJson(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)
	c.Json(200, map[string]interface{}{
		"message": "hello",
	})

	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestContextSetAndGet(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)
	c.Set("key", "value")

	val, exists := c.Get("key")
	if !exists {
		t.Error("Expected key to exist")
	}

	if val != "value" {
		t.Errorf("Expected value 'value', got '%v'", val)
	}
}

func TestContextParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/users/123", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)
	c.Params["id"] = "123"

	if c.Param("id") != "123" {
		t.Errorf("Expected param '123', got '%s'", c.Param("id"))
	}
}

func TestContextRedirect(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)
	c.Redirect(302, "/target")

	if w.Code != 302 {
		t.Errorf("Expected status 302, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/target" {
		t.Errorf("Expected Location '/target', got '%s'", location)
	}
}

func TestContextAbort(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)
	c.Abort()

	if !c.IsAborted() {
		t.Error("Expected context to be aborted")
	}
}

func TestContextNext(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)

	order := ""
	c.handlers = []HandlerFunc{
		func(c *Context) {
			order += "1"
			c.Next()
			order += "4"
		},
		func(c *Context) {
			order += "2"
			c.Next()
			order += "3"
		},
	}

	c.index = -1
	c.Next()

	if order != "1234" {
		t.Errorf("Expected order '1234', got '%s'", order)
	}
}

func TestContextGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected string
	}{
		{
			name:     "X-Forwarded-For",
			headers:  map[string]string{"X-Forwarded-For": "192.168.1.1"},
			expected: "192.168.1.1",
		},
		{
			name:     "X-Real-IP",
			headers:  map[string]string{"X-Real-IP": "10.0.0.1"},
			expected: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()

			c := NewContext(w, req)
			ip := c.GetClientIP()

			if ip != tt.expected {
				t.Errorf("Expected IP '%s', got '%s'", tt.expected, ip)
			}
		})
	}
}

func TestContextReset(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := NewContext(w, req)
	c.Set("key", "value")
	c.Params["id"] = "123"

	newReq := httptest.NewRequest("GET", "/new", nil)
	newW := httptest.NewRecorder()
	c.Reset(newW, newReq)

	if len(c.Params) != 0 {
		t.Error("Expected params to be reset")
	}

	if c.keys != nil {
		t.Error("Expected keys to be nil after reset")
	}
}
