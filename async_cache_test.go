package cache

import (
	"sync"
	"testing"
)

func Test_keyStatus_Set(t *testing.T) {

	type fields struct {
		keyMap map[string]status
		mu     *sync.RWMutex
	}
	type args struct {
		key    string
		status status
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantStatus status
	}{
		{
			name:   "With Valid key and state",
			fields: fields(*NewKeyStatus()),
			args: args{
				key:    "prof_123",
				status: STATUS_INPROCESS,
			},
			wantStatus: STATUS_INPROCESS,
		},
		{
			name:   "With InValid key and state",
			fields: fields(*NewKeyStatus()),
			args: args{
				key:    "",
				status: STATUS_INPROCESS,
			},
			wantStatus: STATUS_INVALID_KEY,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ks := NewKeyStatus()
			t.Parallel()
			ks.Set(tt.args.key, tt.args.status)
			if ks.Get(tt.args.key) != tt.wantStatus {
				t.Log(tt.fields.keyMap)
				t.Errorf("KeyStatys.Set() sets status %v, want status %v", tt.args.status, tt.wantStatus)
			}
		})
	}
}

func Test_keyStatus_Get(t *testing.T) {

	type args struct {
		key string
	}
	tests := []struct {
		name string
		KeyS keyStatus
		args args
		want status
	}{
		{
			name: "With Valid key and state as DONE",
			KeyS: func() keyStatus {
				ks := NewKeyStatus()
				ks.Set("prof_123", STATUS_DONE)
				return *ks
			}(),
			args: args{
				key: "prof_123",
			},
			want: STATUS_DONE,
		},
		{
			name: "With Valid key and state as INPROCESS",
			KeyS: func() keyStatus {
				ks := NewKeyStatus()
				ks.Set("prof_123", STATUS_INPROCESS)
				return *ks
			}(),

			args: args{
				key: "prof_123",
			},
			want: STATUS_INPROCESS,
		},
		{
			name: "With Valid key but not present in keyMap",
			KeyS: *NewKeyStatus(),
			args: args{
				key: "getAdUnit_5890",
			},
			want: STATUS_NOTPRESENT,
		},
		{
			name: "With Invalid key and state as INPROCESS",
			KeyS: *NewKeyStatus(),
			args: args{
				key: "",
			},
			want: STATUS_INVALID_KEY,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ks := NewKeyStatus()
			ks.keyMap = tt.KeyS.keyMap
			t.Parallel()
			if got := ks.Get(tt.args.key); got != tt.want {
				t.Errorf("keyStatus.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}
