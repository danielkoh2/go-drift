package accounts

import (
	"database/sql"
	"driftgo/lib/database"
	"fmt"
)

type UserStatsRecord struct {
	Id      int64
	Address string
	Data    string
}

type UserStatsService struct {
	db    *sql.DB
	table string
}

func NewUserStatsService(db *sql.DB, table string) *UserStatsService {
	service := &UserStatsService{
		db:    db,
		table: table,
	}
	service.init()
	return service
}

func (p *UserStatsService) init() {
	_, err := p.db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, address VARCHAR(64) NOT NULL UNIQUE, data TEXT)", p.table))
	if err != nil {
		panic(err)
	}
}
func (p *UserStatsService) Insert(rows []*UserStatsRecord) (ret bool) {
	defer func() {
		r := recover()
		if r != nil {
			//spew.Dump(r)
			ret = false
		}
	}()
	query := fmt.Sprintf("Insert Into %s(address, data) values ", p.table)
	for index, row := range rows {
		values := fmt.Sprintf("('%s', '%s')", row.Address, row.Data)
		if index == 0 {
			query += values
		} else {
			query += ", " + values
		}
	}
	database.Insert(p.db, query)
	ret = true
	return ret
}

func (p *UserStatsService) Fetch(pubkey string) *UserStatsRecord {
	query := fmt.Sprintf(
		"select id, address, data from %s where address = '%s'",
		p.table, pubkey)
	res := database.SelectOne(p.db, query)
	if res.Err() != nil {
		return nil
	}
	var row UserStatsRecord
	err := res.Scan(&row.Id, &row.Address, &row.Data)
	if err != nil {
		return nil
	}
	return &row

}
func (p *UserStatsService) FetchAll() []*UserStatsRecord {
	query := fmt.Sprintf(
		"select id, address, data from %s",
		p.table)
	res := database.Select(p.db, query)
	var rows []*UserStatsRecord
	for res.Next() {
		var row UserStatsRecord
		err := res.Scan(&row.Id, &row.Address, &row.Data)
		if err != nil {
			panic(err)
		}
		rows = append(rows, &row)
	}
	return rows
}
