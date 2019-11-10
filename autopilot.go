package main

import "time"

type AutopilotContact struct {
	Email     string       `json:"Email"`
	FirstName string       `json:"FirstName"`
	LastName  string       `json:"LastName"`
	Custom    CustomFields `json:"custom"`
}

type CustomFields struct {
	StepOne             time.Time `json:"date--Step--One"`
	StepTwo             time.Time `json:"date--Step--Two"`
	StepThree           time.Time `json:"date--Step--Three"`
	StepFour            time.Time `json:"date--Step--Four"`
	GrowthTrackGraduate bool      `json:"boolean--Growth--Track--Graduate"`
}
