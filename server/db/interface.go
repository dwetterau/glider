package db

import (
	"database/sql"
	"errors"
	"time"

	"github.com/dwetterau/glider/server/types"
	_ "github.com/mattn/go-sqlite3"
)

type Database interface {
	AddOrGetUser(fbID string, timezone *time.Location) (types.UserID, *time.Location, bool, error)
	SetTimezone(userID types.UserID, tz *time.Location) error
	AddOrUpdateActivity(userID types.UserID, activity types.Activity) (types.ActivityID, error)
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
		activityTableTypeDayIndexCreateSchema,
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
fb_id TEXT NOT NULL,
timezone TEXT NOT NULL
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
raw_messages TEXT NOT NULL,
duration INTEGER NOT NULL,
count INTEGER NOT NULL
)
`

const activityTableIndexCreateSchema = `
CREATE INDEX IF NOT EXISTS owner_idx ON activity (user_id, date)
`

const activityTableTypeDayIndexCreateSchema = `
CREATE UNIQUE INDEX IF NOT EXISTS owner_type_date_idx ON activity (user_id, type, date)
`

type databaseImpl struct {
	db *sql.DB
}

var _ Database = &databaseImpl{}

func (d *databaseImpl) AddOrGetUser(fbID string, timezone *time.Location) (types.UserID, *time.Location, bool, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, nil, false, err
	}
	q, err := tx.Prepare("SELECT id, timezone FROM users WHERE fb_id = ?")
	if err != nil {
		return 0, nil, false, err
	}
	rows, err := q.Query(fbID)
	if err != nil {
		return 0, nil, false, err
	}
	for rows.Next() {
		var userID types.UserID
		var timezoneRaw string
		err = rows.Scan(&userID, &timezoneRaw)
		if err != nil {
			return 0, nil, false, err
		}
		err = tx.Rollback()
		if err != nil {
			return 0, nil, false, err
		}
		timezone, err := time.LoadLocation(timezoneRaw)
		if err != nil {
			return 0, nil, false, err
		}
		return userID, timezone, false, nil
	}

	// Otherwise, we need to insert the user
	q, err = tx.Prepare("INSERT INTO users (fb_id, timezone) VALUES (?, ?)")
	if err != nil {
		return 0, nil, false, err
	}
	res, err := q.Exec(fbID, timezone.String())
	if err != nil {
		return 0, nil, false, err
	}

	lastInsertID, err := res.LastInsertId()
	if err != nil {
		return 0, nil, false, err
	}
	err = tx.Commit()
	if err != nil {
		return 0, nil, false, err
	}
	return types.UserID(lastInsertID), timezone, true, nil
}

func (d *databaseImpl) SetTimezone(userID types.UserID, timezone *time.Location) error {
	q, err := d.db.Prepare("UPDATE users SET timezone = ? WHERE id = ?")
	if err != nil {
		return err
	}
	res, err := q.Exec(timezone.String(), userID)
	if err != nil {
		return err
	}
	numChanged, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if numChanged != 1 {
		return errors.New("changed not enough or too many rows")
	}
	return nil
}

func (d *databaseImpl) AddOrUpdateActivity(userID types.UserID, activity types.Activity) (types.ActivityID, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	// Now read and see if the activity currently exists already
	q, err := tx.Prepare("SELECT id FROM activity WHERE user_id = ? AND type = ? AND date = ?")
	if err != nil {
		return 0, err
	}
	rows, err := q.Query(userID, activity.Type, activity.UTCDate.Unix())
	if err != nil {
		return 0, err
	}
	existingID := types.ActivityID(-1)
	for rows.Next() {
		err = rows.Scan(&existingID)
		if err != nil {
			return 0, err
		}
	}
	if existingID != -1 {
		q, err = tx.Prepare("UPDATE activity SET " +
			"time = ?, value = ?, raw_messages = ?, duration = ?, count = ?" +
			"WHERE id = ?")
		if err != nil {
			return 0, err
		}
		res, err := q.Exec(
			activity.ActualTime.Unix(),
			activity.Value,
			activity.RawMessages,
			activity.Duration.Nanoseconds(),
			activity.Count,
			existingID,
		)
		if err != nil {
			return 0, err
		}
		if num, err := res.RowsAffected(); err != nil || num != 1 {
			if err != nil {
				return 0, err
			}
			return 0, errors.New("unable to update activity")
		}
		err = tx.Commit()
		if err != nil {
			return 0, err
		}
		return existingID, nil
	}
	q, err = tx.Prepare("INSERT INTO activity " +
		"(user_id, type, date, time, value, raw_messages, duration, count) " +
		"VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return 0, err
	}
	res, err := q.Exec(
		userID,
		activity.Type,
		activity.UTCDate.Unix(),
		activity.ActualTime.Unix(),
		activity.Value,
		activity.RawMessages,
		activity.Duration.Nanoseconds(),
		activity.Count,
	)
	if err != nil {
		return 0, err
	}
	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	lastInsertID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return types.ActivityID(lastInsertID), nil
}

func (d *databaseImpl) ActivityForUser(userID types.UserID) ([]types.Activity, error) {
	// TODO: Pagination
	activities := make([]types.Activity, 0)
	q, err := d.db.Prepare("SELECT " +
		"id, type, date, time, value, raw_messages, duration, count " +
		"FROM activity where user_id = ? ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		a := types.Activity{}
		var dateRaw, timeRaw, durationRaw int64
		err = rows.Scan(
			&a.ID,
			&a.Type,
			&dateRaw,
			&timeRaw,
			&a.Value,
			&a.RawMessages,
			&durationRaw,
			&a.Count,
		)
		if err != nil {
			return nil, err
		}
		// Parse the dates properly
		a.UTCDate = time.Unix(dateRaw, 0)
		a.ActualTime = time.Unix(timeRaw, 0)
		a.Duration = time.Duration(durationRaw)

		activities = append(activities, a)
	}
	return activities, nil
}
