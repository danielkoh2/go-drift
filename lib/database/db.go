package database

import (
	"database/sql"
	"fmt"
)

func OpenDb(host string, port int, user string, password string, dbName string) *sql.DB {
	dns := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, password, host, port, dbName)
	db, err := sql.Open("mysql", dns)
	if err != nil {
		panic(err)
	}
	return db
}

func CloseDb(db *sql.DB) {
	if db == nil {
		return
	}
	err := db.Close()
	if err != nil {
		panic(err)
	}
}

func Insert(db *sql.DB, query string) {
	//sql := "INSERT INTO cities(name, population) VALUES ('Moscow', 12506000)"
	_, err := db.Exec(query)
	if err != nil {
		panic(err)
	}
}

func SelectOne(db *sql.DB, query string) *sql.Row {
	res := db.QueryRow(query)
	return res
}
func Select(db *sql.DB, query string) *sql.Rows {
	res, err := db.Query(query)
	if err != nil {
		panic(err)
	}
	return res
	//for res.Next() {
	//
	//	var city City
	//	err := res.Scan(&city.Id, &city.Name, &city.Population)
	//
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	fmt.Printf("%v\n", city)
	//}
}

func Delete(db *sql.DB, query string) {
	_, err := db.Exec(query)
	if err != nil {
		panic(err)
	}
}
