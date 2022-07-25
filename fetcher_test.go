package cache

import (
	"reflect"
	"testing"
)

func Test_fetcher_Register(t *testing.T) {
	type fields struct {
		cb        map[string]callbackFunc
		prefixLen int
	}
	type args struct {
		keyPrefix string
		cbf       callbackFunc
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		// TODO: Add test cases.
		{
			name:   "Registering Invalid KeyPrefix",
			fields: fields(*NewFetcher(5)),
			args: args{
				keyPrefix: "AAG0022222",
				cbf:       nil,
			},
			want: false,
		},
		{
			name:   "Registering Empty KeyPrefix",
			fields: fields(*NewFetcher(5)),
			args: args{
				keyPrefix: "",
				cbf:       nil,
			},
			want: false,
		},
		{
			name:   "Registration Valid KeyPrefix",
			fields: fields(*NewFetcher(5)),
			args: args{
				keyPrefix: "AAG00",
				cbf:       CbGetAdUnitConfig,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fetcher{
				cb:        tt.fields.cb,
				prefixLen: tt.fields.prefixLen,
			}
			if got := f.Register(tt.args.keyPrefix, tt.args.cbf); got != tt.want {
				t.Errorf("fetcher.Register() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
				cb: map[string]callbackFunc{
					"AAG00": CbGetAdUnitConfig,
					"AAA00": nil,
					"AAB00": nil,
				},
				prefixLen: 5,
			},
			args: args{
				key: "AAG00_5890",
			},
			want:    "AdUnitConfig",
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

func CbGetAdUnitConfig(key string) (interface{}, error) {
	//Spliting Key to Call Respective DB call
	//info := strings.Split(key, "_")

	//profileID, _ := strconv.Atoi(info[1])
	//displayVersionID, _ := strconv.Atoi(info[2])

	data := "AdUnitConfig"

	return data, nil

}
