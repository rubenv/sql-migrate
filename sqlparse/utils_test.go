package sqlparse

import (
	"testing"
)

func TestParseSQL(t *testing.T) {
	var tests = []struct {
		input string
		want  alterQuery
	}{
		{
			input: "ALTER TABLE `Box` ADD COLUMN `column` INT NOT NULL DEFAULT 0;",
			want: alterQuery{
				Action: "ADD COLUMN `column` INT NOT NULL DEFAULT 0",
				Table:  "Box",
			},
		},
		{
			input: "ALTER TABLE User DROP COLUMN profile;",
			want: alterQuery{
				Action: "DROP COLUMN profile",
				Table:  "User",
			},
		},
		{
			input: `
			ALTER TABLE User DROP COLUMN profile;
			`,
			want: alterQuery{
				Action: "DROP COLUMN profile",
				Table:  "User",
			},
		},
		{
			input: `
				UPDATE SubscriptionLog SET userVendorToken = "" WHERE userVendorToken IS NULL;
				`,
			want: alterQuery{},
		},
		{
			input: "ALTER TABLE  `Contract`  MODIFY city varchar(50) NOT NULL DEFAULT '';",
			want: alterQuery{
				Action: "MODIFY city varchar(50) NOT NULL DEFAULT ''",
				Table:  "Contract",
			},
		},
	}
	for _, test := range tests {
		if _, got := ParseAlterQuery(test.input); got != test.want {
			t.Error(got)
		}
	}
}
