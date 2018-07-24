package sqlparse

import "testing"

func TestParseSQL(t *testing.T) {
	var tests = []struct {
		input string
		want  alterQuery
	}{
		{"ALTER TABLE `Box` ADD COLUMN `column` INT NOT NULL DEFAULT 0;", alterQuery{Action: "ADD COLUMN `column` INT NOT NULL DEFAULT 0", Table: "Box"}},
		{"ALTER TABLE User DROP COLUMN profile;", alterQuery{Action: "DROP COLUMN profile", Table: "User"}},
		{`
			ALTER TABLE User DROP COLUMN profile;
			`, alterQuery{Action: "DROP COLUMN profile", Table: "User"}},
		{`
				UPDATE SubscriptionLog SET userVendorToken = "" WHERE userVendorToken IS NULL;
				`, alterQuery{}},
	}
	for _, test := range tests {
		if _, got := ParseAlterQuery(test.input); got != test.want {
			t.Error(got)
		}
	}
}
