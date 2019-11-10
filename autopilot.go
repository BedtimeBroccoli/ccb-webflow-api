package main

import "time"

type AutopilotContact struct {
	Email     string       `json:"Email"`
	FirstName string       `json:"FirstName"`
	LastName  string       `json:"LastName"`
	Custom    CustomFields `json:"custom"`
}

type CustomFields struct {
	StepOne             *time.Time `json:"date--Step--One,omitempty"`
	StepTwo             *time.Time `json:"date--Step--Two,omitempty"`
	StepThree           *time.Time `json:"date--Step--Three,omitempty"`
	StepFour            *time.Time `json:"date--Step--Four,omitempty"`
	GrowthTrackGraduate bool       `json:"boolean--Growth--Track--Graduate,omitempty"`
}
