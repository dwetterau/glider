package conversation

import (
	"github.com/dwetterau/glider/server/db"
)

type Manager interface {
	Handle(fbID string, message string) string
}

func New(
	database db.Database,
	client WitClient,
) Manager {
	m := &managerImpl{
		database:        database,
		witClient:       client,
		currentMessages: make(map[string]*state, 10),
	}
	m.start()
	return m
}
