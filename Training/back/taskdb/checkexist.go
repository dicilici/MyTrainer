package taskdb

import (
	"encoding/json"
	"fmt"
	bolt "go.etcd.io/bbolt"
	"os"
	"time"
	"train/pkg"
)

func CheckExist(t time.Time, actives []*bolt.DB, key string, path string) bool {
	ShouldRemove := false
	exist := true
	i := -1
	for index, active := range actives {
		if t.Equal(pkg.GetTime(active.Path())) {
			i = index
		}
	}
	switch i {
	case -1:
		path := fmt.Sprint(path+"/taskdatabase/", t.Format("2006-01-02 15:04:05"))
		db, _ := bolt.Open(path, 0600, nil)
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("tasks"))
			if b == nil {
				ShouldRemove = true
				exist = false
				return nil
			}
			data := b.Get([]byte(key))
			if data == nil {
				exist = false
				return nil
			}
			var taskstatus TaskStatus
			json.Unmarshal(data, &taskstatus)
			if taskstatus.Deleted == true {
				exist = false
				return nil
			}
			return nil
		})
		db.Close()
		if ShouldRemove == true {
			os.Remove(path)
		}
	default:
		actives[i].View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("tasks"))
			if b == nil {
				exist = false
				return nil
			}
			data := b.Get([]byte(key))
			if data == nil {
				exist = false
				return nil
			}
			var taskstatus TaskStatus
			json.Unmarshal(data, &taskstatus)
			if taskstatus.Deleted == true {
				exist = false
				return nil
			}
			return nil
		})
	}
	return exist
}
