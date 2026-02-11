package models

import "testing"

func TestDenyListIsDenied(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		list []string
		ip   string
		want bool
	}{
		{
			name: "empty list",
			list: nil,
			ip:   "10.0.0.1",
			want: false,
		},
		{
			name: "match",
			list: []string{"10.0.0.1", "10.0.0.2"},
			ip:   "10.0.0.2",
			want: true,
		},
		{
			name: "no match",
			list: []string{"10.0.0.1"},
			ip:   "10.0.0.3",
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := &DenyList{list: tt.list}
			if got := d.IsDenied(tt.ip); got != tt.want {
				t.Fatalf("IsDenied(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
