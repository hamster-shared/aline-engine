package utils

import "testing"

func TestGetJobNameAndIDFromFormatString(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   int
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				str: "test1(1)",
			},
			want:    "test1",
			want1:   1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := GetJobNameAndIDFromFormatString(tt.args.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJobNameAndIDFromFormatString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetJobNameAndIDFromFormatString() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetJobNameAndIDFromFormatString() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
