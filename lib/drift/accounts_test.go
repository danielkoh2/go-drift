package drift

import (
	"github.com/davecgh/go-spew/spew"
	"testing"
)

// go test --run TestParseAnyAccount

func TestParseAnyAccount(t *testing.T) {
	spew.Dump("ParseAnyAccount")
	data := TestData
	accountData, err := ParseAnyAccount(data)
	if err != nil {
		t.Fatal(err)
	}
	spew.Dump("TestParseAnyAccount Result", accountData)
}

// go test --run TestParseAccountUser

func TestParseAccountUser(t *testing.T) {
	spew.Dump("TestParseAccountUser")
	data := TestData
	accountData, err := ParseAccount_User(data)
	if err != nil {
		t.Fatal(err)
	}
	spew.Dump("TestParseAccountUser Result", accountData)
}

// go test --run TestParseAccountUserStats

func TestParseAccountUserStats(t *testing.T) {
	spew.Dump("TestParseAccount_UserStats")
	data := TestData
	accountData, err := ParseAccount_UserStats(data)
	if err != nil {
		t.Fatal(err)
	}
	spew.Dump("TestParseAccount_UserStats Result", accountData)
}
