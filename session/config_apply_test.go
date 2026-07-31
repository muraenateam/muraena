package session

import (
	"reflect"
	"sort"
	"testing"
)

func TestCollectPaths(t *testing.T) {
	patch := map[string]interface{}{
		"Proxy": map[string]interface{}{"Port": 8080},
		"Transform": map[string]interface{}{
			"Response": map[string]interface{}{"CustomContent": []interface{}{}},
		},
	}
	got := CollectPaths(patch)
	sort.Strings(got)
	want := []string{"Proxy.Port", "Transform.Response.CustomContent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRestartRequiredFields(t *testing.T) {
	rr := RestartRequiredFields([]string{"Proxy.Port", "Transform.Request.UserAgent", "TLS.Certificate"})
	sort.Strings(rr)
	want := []string{"Proxy.Port", "TLS.Certificate"}
	if !reflect.DeepEqual(rr, want) {
		t.Fatalf("got %v want %v", rr, want)
	}
}
