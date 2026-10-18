package endpoint

import (
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"root", "https://api.example.com", "/"},
		{"root with slash", "https://api.example.com/", "/"},
		{"plain path", "https://api.example.com/login", "/login"},
		{"query string ignored", "https://api.example.com/products?page=2&sort=asc", "/products"},
		{"fragment ignored", "https://api.example.com/products#top", "/products"},
		{"trailing slash ignored", "https://api.example.com/checkout/", "/checkout"},
		{"duplicate slashes collapsed", "https://api.example.com//a///b", "/a/b"},
		{"numeric id", "https://api.example.com/orders/12345", "/orders/{id}"},
		{"uuid", "https://api.example.com/users/123e4567-e89b-12d3-a456-426614174000/profile", "/users/{id}/profile"},
		{"object id", "https://api.example.com/docs/507f1f77bcf86cd799439011", "/docs/{id}"},
		{"counter token", "https://api.example.com/orders/{{counter}}", "/orders/{id}"},
		{"random token", "https://api.example.com/items/{{random}}/reviews", "/items/{id}/reviews"},
		{"token inside segment", "https://api.example.com/export/order-{{counter}}.csv", "/export/order-{id}.csv"},
		{"short hex word kept", "https://api.example.com/cafe/face", "/cafe/face"},
		{"words kept", "https://api.example.com/v1/users/me", "/v1/users/me"},
		{"case preserved", "https://api.example.com/Admin/Users", "/Admin/Users"},
		{"scheme-less path with query", "/search?q=go", "/search"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.url); got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestLabelsMergeDynamicURLsOfOneEndpoint(t *testing.T) {
	urls := []string{
		"https://api.example.com/orders/{{counter}}",
		"https://api.example.com/orders/7?expand=items",
		"https://api.example.com/login",
	}
	got := Labels(urls)
	want := []string{"/orders/{id}", "/orders/{id}", "/login"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Labels() = %v, want %v", got, want)
	}
	if distinct := Distinct(got); !reflect.DeepEqual(distinct, []string{"/login", "/orders/{id}"}) {
		t.Fatalf("Distinct() = %v", distinct)
	}
}

func TestLabelsKeepHostOnlyWhenHostsDiffer(t *testing.T) {
	same := Labels([]string{"https://a.example.com/x", "https://A.example.com/y"})
	if !reflect.DeepEqual(same, []string{"/x", "/y"}) {
		t.Errorf("same host labels = %v, want path only", same)
	}

	mixed := Labels([]string{"https://a.example.com/x", "https://b.example.com/x"})
	if !reflect.DeepEqual(mixed, []string{"a.example.com/x", "b.example.com/x"}) {
		t.Errorf("mixed host labels = %v, want host-prefixed", mixed)
	}
	if len(Distinct(mixed)) != 2 {
		t.Errorf("same path on different hosts should stay distinct")
	}
}

func TestLabelsEmpty(t *testing.T) {
	if got := Labels(nil); len(got) != 0 {
		t.Errorf("Labels(nil) = %v, want empty", got)
	}
}
