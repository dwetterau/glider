package db

import (
	"database/sql"
	"time"

	"github.com/dwetterau/glider/server/types"
	_ "github.com/mattn/go-sqlite3"
)

type Database interface {
	AddUser(fbID string) (types.UserID, error)
	AddActivity(userID types.UserID, activity types.Activity) (types.ActivityID, error)
	ActivityForUser(userID types.UserID) ([]types.Activity, error)
}

func NewSQLite(sourcePath string) (Database, error) {
	database, err := sql.Open("sqlite3", sourcePath)
	if err != nil {
		return nil, err
	}
	// Set up all the needed tables
	for _, schema := range []string{
		userTableCreateSchema,
		userTableIndexCreateShchema,
		activityTableCreateSchema,
		activityTableIndexCreateSchema,
	} {
		statement, err := database.Prepare(schema)
		if err != nil {
			return nil, err
		}
		_, err = statement.Exec()
		if err != nil {
			return nil, err
		}
	}

	return &databaseImpl{db: database}, nil
}

const userTableCreateSchema = `
CREATE TABLE IF NOT EXISTS users (
id INTEGER PRIMARY KEY, 
fb_id TEXT NOT NULL
)`

const userTableIndexCreateShchema = `
CREATE UNIQUE INDEX IF NOT EXISTS fb_id_idx ON users (fb_id)
`

const activityTableCreateSchema = `
CREATE TABLE IF NOT EXISTS activity (
id INTEGER PRIMARY KEY,
user_id INTEGER NOT NULL,
type INTEGER NOT NULL,
date INTEGER NOT NULL,
time INTEGER NOT NULL,
value TEXT NOT NULL,
raw_value TEXT NOT NULL
)
`

const activityTableIndexCreateSchema = `
CREATE INDEX IF NOT EXISTS owner_idx ON activity (user_id, date)
`

type databaseImpl struct {
	db *sql.DB
}

var _ Database = &databaseImpl{}

func (d *databaseImpl) AddUser(fbID string) (types.UserID, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	q, err := tx.Prepare("SELECT id FROM users WHERE fb_id = ?")
	if err != nil {
		return 0, err
	}
	rows, err := q.Query(fbID)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var userID types.UserID
		err = rows.Scan(&userID)
		if err != nil {
			return 0, err
		}
		err = tx.Rollback()
		if err != nil {
			return 0, err
		}
		return userID, nil
	}

	// Otherwise, we need to insert the user
	q, err = tx.Prepare("INSERT INTO users (fb_id) VALUES (?)")
	if err != nil {
		return 0, err
	}
	res, err := q.Exec(fbID)
	if err != nil {
		return 0, err
	}

	lastInsertID, err := res.LastInsertId()
	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	return types.UserID(lastInsertID), nil
}

func (d *databaseImpl) AddActivity(userID types.UserID, activity types.Activity) (types.ActivityID, error) {
	q, err := d.db.Prepare("INSERT INTO activity " +
		"(user_id, type, date, time, value, raw_value) " +
		"VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	res, err := q.Exec(
		userID,
		activity.Type,
		activity.UTCDate.Unix(),
		activity.ActualTime.Unix(),
		activity.Value,
		activity.RawValue,
	)
	if err != nil {
		return 0, err
	}
	lastInsertID, err := res.LastInsertId()
	return types.ActivityID(lastInsertID), nil
}

func (d *databaseImpl) ActivityForUser(userID types.UserID) ([]types.Activity, error) {
	// TODO: Pagination
	activities := make([]types.Activity, 0)
	q, err := d.db.Prepare("SELECT id, type, date, time, value, raw_value FROM activity where user_id = ? ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		a := types.Activity{}
		var dateRaw, timeRaw int64
		err = rows.Scan(
			&a.ID,
			&a.Type,
			&dateRaw,
			&timeRaw,
			&a.Value,
			&a.RawValue,
		)
		if err != nil {
			return nil, err
		}
		// Parse the dates properly
		a.UTCDate = time.Unix(dateRaw, 0)
		a.ActualTime = time.Unix(timeRaw, 0)

		activities = append(activities, a)
	}
	return activities, nil
}
