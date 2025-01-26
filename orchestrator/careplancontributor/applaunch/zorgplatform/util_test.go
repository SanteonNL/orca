package zorgplatform

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_xmlTimeBetweenInclusive(t *testing.T) {
	type args struct {
		notBefore string
		notAfter  string
		check     time.Time
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "between",
			args: args{
				notBefore: "2021-09-01T00:00:00Z",
				notAfter:  "2021-09-30T23:59:59Z",
				check:     time.Date(2021, 9, 15, 0, 0, 0, 0, time.UTC),
			},
			want: true,
		},
		{
			name: "before",
			args: args{
				notBefore: "2021-09-01T00:00:00Z",
				notAfter:  "2021-09-30T23:59:59Z",
				check:     time.Date(2021, 8, 15, 0, 0, 0, 0, time.UTC),
			},
			want: false,
		},
		{
			name: "after",
			args: args{
				notBefore: "2021-09-01T00:00:00Z",
				notAfter:  "2021-09-30T23:59:59Z",
				check:     time.Date(2021, 10, 15, 0, 0, 0, 0, time.UTC),
			},
			want: false,
		},
		{
			name: "equal to notBefore",
			args: args{
				notBefore: "2021-09-01T00:00:00Z",
				notAfter:  "2021-09-30T23:59:59Z",
				check:     time.Date(2021, 9, 1, 0, 0, 0, 0, time.UTC),
			},
			want: true,
		},
		{
			name: "equal to notAfter",
			args: args{
				notBefore: "2021-09-01T00:00:00Z",
				notAfter:  "2021-09-30T23:59:59Z",
				check:     time.Date(2021, 9, 30, 23, 59, 59, 0, time.UTC),
			},
		},
		{
			name: "invalid notBefore",
			args: args{
				notBefore: "invalid",
				notAfter:  "2021-09-30T23:59:59Z",
				check:     time.Date(2021, 9, 15, 0, 0, 0, 0, time.UTC),
			},
			wantErr: assert.Error,
		},
		{
			name: "invalid notAfter",
			args: args{
				notBefore: "2021-09-01T00:00:00Z",
				notAfter:  "invalid",
				check:     time.Date(2021, 9, 15, 0, 0, 0, 0, time.UTC),
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := xmlTimeBetweenInclusive(tt.args.notBefore, tt.args.notAfter, tt.args.check)
			if !tt.wantErr(t, err, fmt.Sprintf("xmlTimeBetweenInclusive(%v, %v, %v)", tt.args.notBefore, tt.args.notAfter, tt.args.check)) {
				return
			}
			assert.Equalf(t, tt.want, got, "xmlTimeBetweenInclusive(%v, %v, %v)", tt.args.notBefore, tt.args.notAfter, tt.args.check)
		})
	}
}
