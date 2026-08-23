package adapters

import "testing"

// The permission bypass used to be hardcoded into every adapter's argv, so
// model.auto_approve_tools had no effect. AutoApprove must fail closed.
func TestAutoApprove(t *testing.T) {
	cases := []struct {
		name    string
		options map[string]string
		want    bool
	}{
		{"nil options", nil, false},
		{"absent key", map[string]string{"effort": "high"}, false},
		{"empty value", map[string]string{"auto_approve": ""}, false},
		{"explicit false", map[string]string{"auto_approve": "false"}, false},
		{"garbage", map[string]string{"auto_approve": "maybe"}, false},
		{"true", map[string]string{"auto_approve": "true"}, true},
		{"one", map[string]string{"auto_approve": "1"}, true},
		{"yes", map[string]string{"auto_approve": "yes"}, true},
	}
	for _, tc := range cases {
		if got := AutoApprove(tc.options); got != tc.want {
			t.Errorf("%s: AutoApprove = %v, want %v", tc.name, got, tc.want)
		}
	}
}
