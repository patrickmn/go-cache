package cache

import (
	"testing"
)

func TestSet(t *testing.T) {

	type args struct {
		key string
		S   state
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Default",
			args: args{
				key: "prof_123",
				S:   INPROCESS,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ks KeyState
			if got := ks.Set(tt.args.key, tt.args.S); got != tt.want {
				t.Errorf("Set() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyState_Get(t *testing.T) {
	type args struct {
		key string
	}
	tests := []struct {
		name string
		ks   KeyState
		args args
		want state
	}{
		{
			name: "Default",
			ks: func() KeyState {
				var ks KeyState
				ks.Set("prof_id_123", INPROCESS)
				return ks
			}(),
			args: args{
				key: "prof_123",
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ks KeyState
			ks.Set("prof_id_123", INPROCESS)
			if got := tt.ks.Get(tt.args.key); got != tt.want {
				t.Errorf("KeyState.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}
