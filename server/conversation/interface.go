package conversation

import (
	"glider/server/db"
)

type Manager interface {
	Handle(fbID string, message string) string
}

func New(
	database db.Database,
) Manager {
	m := &managerImpl{
		database:        database,
		currentMessages: make(map[string]*state, 10),
	}
	m.start()
	return m
}
