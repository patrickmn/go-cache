package cache

import (
	"reflect"
	"testing"
)

func Test_fetcher_Execute(t *testing.T) {
	type fields struct {
		cb        map[string]callbackFunc
		prefixLen int
	}
	type args struct {
		key string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Invalid Key",
			fields: fields{
				cb: map[string]callbackFunc{
					"AAG00": nil,
				},
				prefixLen: 5,
			},
			args: args{
				key: "INV",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Unexisting Key Execution",
			fields: fields{
				cb: map[string]callbackFunc{
					"AAG00": nil,
					"AAA00": nil,
					"AAB00": nil,
				},
				prefixLen: 5,
			},
			args: args{
				key: "UnExisted_Key",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid Key Execution",
			fields: fields{
				cb:        make(map[string]callbackFunc),
				prefixLen: 5,
			},
			args: args{
				key: "AAG00_5890",
			},
			want: f.cb{
				"afewv",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fetcher{
				cb:        tt.fields.cb,
				prefixLen: tt.fields.prefixLen,
			}
			got, err := f.Execute(tt.args.key)
			// if errors.Is(err, tt.wantErr) {
			// 	t.Errorf("fetcher.Execute() error = %v, wantErr %v", err, tt.wantErr)
			// 	return
			// }
			if (err != nil) != tt.wantErr {
				t.Errorf("fetcher.Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fetcher.Execute() = %v, want %v", got, tt.want)
			}
		})
	}
}
