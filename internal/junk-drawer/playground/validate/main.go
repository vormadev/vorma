package main

import (
	"errors"
	"fmt"

	"github.com/vormadev/vorma/kit/validate"
)

type FirstName string
type LastName string

func (ln LastName) Validate() error {
	return errors.New("asdf")
}

type Job struct {
	Employer string
	Title    string
}

type Email struct {
	EmailAddress *string
}

type Person struct {
	FirstName
	LastName
	*Job
	Email
}

func (p *Person) Validate() error {
	v := validate.Object(p)
	v.Required("FirstName")
	v.Required("LastName")
	v.Required("Employer")
	v.Required("Title")
	v.Required("EmailAddress")
	return v.Error()
}

func main() {
	// lastName := LastName("Cook")
	// // emailAddress := "bob@bob.com

	// p := &Person{
	// 	FirstName: "Samuel",
	// 	LastName:  lastName,
	// 	Job: &Job{
	// 		Employer: "Jim",
	// 		Title:    "",
	// 	},
	// 	Email: Email{
	// 		// EmailAddress: &emailAddress,
	// 	},
	// }

	// err := p.Validate()

	// if err != nil {
	// 	fmt.Println(err)
	// } else {
	// 	fmt.Println("OK")
	// }

	// //////////////

	// var x LastName = "Cook"
	// err = validate.Any("x", x).Required().Error()
	// if err != nil {
	// 	fmt.Println("2", err)
	// } else {
	// 	fmt.Println("2 OK")
	// }

	// err := validate.Any("int", 0).Required().Error()
	// if err != nil {
	// 	fmt.Println("3", err)
	// } else {
	// 	fmt.Println("3 OK")
	// }

	var m map[string]any
	fmt.Println("m", m, m == nil)
	err := validate.Object(m).Error()
	fmt.Println(err)
}

// type Person map[string]string

// func (p *Person) Validate() error {
// 	v := validate.Object(p)
// 	v.Required("FirstName").Min(20)
// 	v.Required("LastName")
// 	return v.Error()
// }

// func main() {
// 	p := &Person{
// 		"FirstName": "Samuel",
// 		"LastName":  "Cook",
// 	}

// 	err := p.Validate()

// 	if err != nil {
// 		fmt.Println(err)
// 	} else {
// 		fmt.Println("OK")
// 	}
// }
