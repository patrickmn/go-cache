package cache

import (
	"sync"
	"testing"
)

func Test_keyStates_Set(t *testing.T) {
	type fields struct {
		keyState map[string]state
		mu       *sync.RWMutex
	}
	type args struct {
		key    string
		status state
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		want       bool
		wantStatus state
		wantErr    error
	}{
		{
			name: "With Valid key and state",
			fields: fields{
				keyState: map[string]state{},
				mu:       &sync.RWMutex{},
			},
			args: args{
				key:    "prof_123",
				status: INPROCESS,
			},
			want:       true,
			wantStatus: INPROCESS,
			wantErr:    nil,
		},
		{
			name: "With InValid key and state",
			fields: fields{
				keyState: map[string]state{},
				mu:       &sync.RWMutex{},
			},
			args: args{
				key:    "",
				status: INPROCESS,
			},
			want:    false,
			wantErr: ErrInvalidKey,
		},
		{
			name: "nil check for keystateSet",
			fields: fields{
				keyState: nil,
				mu:       &sync.RWMutex{},
			},
			args: args{
				key:    "",
				status: INPROCESS,
			},
			want:    false,
			wantErr: ErrKeyStateNotPresent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ks := &keyStates{
				keyState: tt.fields.keyState,
				mu:       tt.fields.mu,
			}
			got, err := ks.Set(tt.args.key, tt.args.status)
			if got != tt.want {
				t.Errorf("keyStates.Set() = %v, want %v", got, tt.want)
			}
			if err != tt.wantErr {
				t.Errorf("keyStates.Set() Error = %v, want %v", err, tt.wantErr)
			}

		})
	}
}

func Test_keyStates_set(t *testing.T) {
	type fields struct {
		keyState map[string]state
		mu       *sync.RWMutex
	}
	type args struct {
		k string
		s state
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "With Valid key and state",
			fields: fields{
				keyState: map[string]state{},
			},
			args: args{
				k: "prof_123",
				s: INPROCESS,
			},
			want: true,
		},
		{
			name: "With Invalid key and  valid state",
			fields: fields{
				keyState: map[string]state{},
			},
			args: args{
				k: "",
				s: INPROCESS,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ks := &keyStates{
				keyState: tt.fields.keyState,
				mu:       tt.fields.mu,
			}
			if got := ks.set(tt.args.k, tt.args.s); got != tt.want {
				t.Errorf("keyStates.set() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_keyStates_get(t *testing.T) {
	type fields struct {
		keyState map[string]state
		mu       *sync.RWMutex
	}
	type args struct {
		k string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   state
		want1  bool
	}{
		{
			name: "With Valid key and  valid state",
			fields: fields{
				keyState: map[string]state{"prof_123": DONE},
			},
			args: args{
				k: "prof_123",
			},
			want:  DONE,
			want1: true,
		},
		{
			name: "With InValid key and  invalid state ",
			fields: fields{
				keyState: map[string]state{},
			},
			args: args{
				k: "",
			},
			want:  NOPRESENT,
			want1: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ks := &keyStates{
				keyState: tt.fields.keyState,
				mu:       tt.fields.mu,
			}
			got, got1 := ks.get(tt.args.k)
			if got != tt.want {
				t.Errorf("keyStates.get() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("keyStates.get() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_keyStates_Get(t *testing.T) {
	type fields struct {
		keyState map[string]state
		mu       *sync.RWMutex
	}
	type args struct {
		key string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    state
		wantErr error
	}{
		{
			name: "With Valid key and state as DONE",
			fields: fields{
				keyState: map[string]state{"prof_123": DONE},
			},
			args: args{
				key: "prof_123",
			},
			want:    DONE,
			wantErr: nil,
		},
		{
			name: "With InValid key and state",
			fields: fields{
				keyState: map[string]state{"prof_123": DONE},
			},
			args: args{
				key: "",
			},
			want:    INVALID,
			wantErr: ErrInvalidKey,
		},
		{
			name: "With Valid key and state as INPROCESS",
			fields: fields{
				keyState: map[string]state{"prof_123": INPROCESS},
			},
			args: args{
				key: "prof_123",
			},
			want:    INPROCESS,
			wantErr: nil,
		},
		{
			name: "nil check",
			fields: fields{
				keyState: nil,
			},
			args: args{
				key: "prof_123",
			},
			want:    INVALID,
			wantErr: ErrKeyStateNotPresent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ks := &keyStates{
				keyState: tt.fields.keyState,
				mu:       tt.fields.mu,
			}
			got, err := ks.Get(tt.args.key)
			if err != tt.wantErr {
				t.Errorf("keyStates.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("keyStates.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}
