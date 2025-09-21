package entities

type Player struct {
	ID int `db:"id" pk:"true"`

	Name bool `db:"name" default:"true"`

	// FullName - has too long index with LongParamNumberOne, LongParamNumberTwo, LongParamNumberThree fields
	//FullName             *string `db:"full_name" index:"idx__player_full_name_long_param_number_one_long_param_number_two_long_param_number_three"`
	//LongParamNumberOne   *int    `db:"long_param_number_one" index:"idx__player_full_name_long_param_number_one_long_param_number_two_long_param_number_three"`
	//LongParamNumberTwo   *int    `db:"long_param_number_two" index:"idx__player_full_name_long_param_number_one_long_param_number_two_long_param_number_three"`
	//LongParamNumberThree *int    `db:"long_param_number_three" index:"idx__player_full_name_long_param_number_one_long_param_number_two_long_param_number_three"`
}
