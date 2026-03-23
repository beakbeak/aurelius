package media

import "github.com/beakbeak/aurelius/internal/mediadb"

// DB exposes the internal database for testing.
func (ml *Library) DB() *mediadb.DB {
	return ml.db
}
