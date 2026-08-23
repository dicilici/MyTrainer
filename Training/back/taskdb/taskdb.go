package taskdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	bolt "go.etcd.io/bbolt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"train/config"
	"train/pkg"
)

type TaskStatus struct {
	C            config.Config
	ErrorMessage string
	Status       string
	Progress     string
	StartTime    time.Time
	EndTime      time.Time
	Deleted      bool
	DeletedAt    time.Time
}

type TaskDb interface {
	Init() error
	CheckActive(t time.Time) bool
	Insert(key string, value TaskStatus, t time.Time) error
	Delete(key string, t time.Time) error
	Update(key string, value TaskStatus, t time.Time) error
	View(key string, t time.Time) (error, string)
	Close() error
	PrintData(b []byte) string
}

type DefaultTaskDb struct {
	LogPath string
	Active  []*bolt.DB
	Number  int
	mux     *sync.RWMutex
	ctx     context.Context
	wg      *sync.WaitGroup
}

func NewDefaultTaskDb(path string, ctx context.Context) *DefaultTaskDb {
	active := make([]*bolt.DB, 0, 10)
	month := pkg.ActiveTime()
	for i := 0; i < 5; i++ {
		path := fmt.Sprint("taskdatabase/", month[i])
		activeTask, err := bolt.Open(path, 0600, &bolt.Options{
			Timeout: 3 * time.Second,
		})
		if err != nil {
			pkg.LogWithString(nil, "task management startup failed", strconv.Itoa(-2), false)
		}
		active[i] = activeTask
	}
	return &DefaultTaskDb{
		LogPath: path,
		Active:  active,
		Number:  0,
		mux:     &sync.RWMutex{},
		ctx:     ctx,
		wg:      &sync.WaitGroup{},
	}
}

func (d *DefaultTaskDb) PrintData(b []byte) string {
	d.wg.Add(1)
	defer d.wg.Done()
	select {
	case <-d.ctx.Done():
		return ""
	default:
		var carrier TaskStatus
		json.Unmarshal(b, &carrier)
		str := fmt.Sprintf("ErrorMessage: %s\nStatus: %s\nProgress: %s\nStartTime: %s\nEndTime: %s\nTaskConfig: %v\n", carrier.ErrorMessage, carrier.Status, carrier.Progress, carrier.StartTime, carrier.EndTime, carrier.C)
		return str
	}
}

func (d *DefaultTaskDb) Init() error {
	d.wg.Add(1)
	defer d.wg.Done()
	select {
	case <-d.ctx.Done():
		return nil
	default:
		file, err := os.OpenFile("/active.txt", os.O_RDONLY, 0644)
		if err != nil {
			pkg.Log(nil, err, strconv.Itoa(-2), false)
			return err
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			pkg.Log(nil, err, strconv.Itoa(-2), false)
		}
		dates := pkg.ReTime(data)
		for index, date := range dates {
			db, _ := bolt.Open(fmt.Sprint("taskdatabase/", date, ".db"), 0600, &bolt.Options{})
			d.Active[index] = db
		}
		d.Number = len(d.Active)
		return nil
	}
}

func (d *DefaultTaskDb) CheckActive(t time.Time) bool {
	d.wg.Add(1)
	defer d.wg.Done()
	select {
	case <-d.ctx.Done():
		return false
	default:
		dates := pkg.ActiveTime()
		for _, active := range dates {
			if t.Equal(pkg.GetTime(active)) {
				return true
			}
		}
		return false
	}
}

func (d *DefaultTaskDb) Insert(key string, value TaskStatus, t time.Time) error {
	d.wg.Add(1)
	defer d.wg.Done()
	select {
	case <-d.ctx.Done():
		return nil
	default:
		NewDB := CheckExist(t, d.Active, key, d.LogPath)
		d.mux.Lock()
		defer d.mux.Unlock()
		IsActive := d.CheckActive(t)
		switch NewDB {
		case false:
			path := fmt.Sprint(d.LogPath+"/taskdatabase/", t.Format("2006-01-02 15:04:05"), ".db")
			db, err := bolt.Open(path, 0600, &bolt.Options{
				Timeout: 3 * time.Second,
			})
			if err != nil {
				pkg.Log(nil, err, strconv.Itoa(-2), false)
				return err
			}
			valueb, _ := json.Marshal(value)
			err = db.Update(func(tx *bolt.Tx) error {
				b, erri := tx.CreateBucketIfNotExists([]byte("tasks"))
				if erri != nil {
					pkg.Log(nil, err, strconv.Itoa(-2), false)
					return erri
				}
				erri = b.Put([]byte(key), valueb)
				if erri != nil {
					pkg.Log(nil, err, strconv.Itoa(-2), false)
					return erri
				}
				return nil
			})
			if err != nil {
				pkg.Log(nil, err, strconv.Itoa(-2), false)
				return err
			}
			d.Number++
			switch IsActive {
			case true:
				d.Active[0].Close()
				d.Active = d.Active[1:]
				d.Active = append(d.Active, db)
			case false:
				db.Close()
			}
		case true:
			path := fmt.Sprint(d.LogPath+"/taskdatabase/", t.Format("2006-01-02"), ".db")
			db, err := bolt.Open(path, 0600, &bolt.Options{
				Timeout: 3 * time.Second,
			})
			if err != nil {
				pkg.Log(nil, err, strconv.Itoa(-2), false)
				return err
			}
			valueb, _ := json.Marshal(value)
			err = db.Update(func(tx *bolt.Tx) error {
				b, erri := tx.CreateBucketIfNotExists([]byte("tasks"))
				if erri != nil {
					pkg.Log(nil, err, strconv.Itoa(-2), false)
					return erri
				}
				erri = b.Put([]byte(key), valueb)
				if erri != nil {
					pkg.Log(nil, err, strconv.Itoa(-2), false)
					return erri
				}
				return nil
			})
			if err != nil {
				pkg.Log(nil, err, strconv.Itoa(-2), false)
				return err
			}
			d.Number++
			if IsActive == false {
				db.Close()
			}
		}
		return nil
	}

}

func (d *DefaultTaskDb) Delete(key string, t time.Time) error {
	d.wg.Add(1)
	defer d.wg.Done()
	select {
	case <-d.ctx.Done():
		return nil
	default:
		IsActive := d.CheckActive(t)
		d.mux.Lock()
		defer d.mux.Unlock()
		IsExist := CheckExist(t, d.Active, key, d.LogPath)
		switch IsExist {
		case true:
			path := fmt.Sprint("taskdatabase/", t.Format("2006-01-02 15:04:05"), ".db")
			db, err := bolt.Open(path, 0600, &bolt.Options{
				Timeout: 3 * time.Second,
			})
			if err != nil {
				pkg.Log(nil, err, strconv.Itoa(-2), false)
				return err
			}
			defer func() {
				if IsActive == false {
					db.Close()
				}
			}()
			err = db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("tasks"))
				data := b.Get([]byte(key))
				var taskstatus TaskStatus
				json.Unmarshal(data, &taskstatus)
				taskstatus.Deleted = true
				taskstatus.DeletedAt = time.Now()
				data, _ = json.Marshal(taskstatus)
				erri := b.Put([]byte(key), data)
				if erri != nil {
					pkg.Log(nil, err, strconv.Itoa(-2), false)
					return erri
				}
				return nil
			})
			if err != nil {
				pkg.Log(nil, err, strconv.Itoa(-2), false)
				return err
			}
			d.Number--
			if IsActive == false {
				db.Close()
			}
		case false:
			err := errors.New("training task not found")
			pkg.Log(nil, err, strconv.Itoa(-2), false)
			return err
		}
		return nil
	}
}

func (d *DefaultTaskDb) Update(key string, value TaskStatus, t time.Time) error {
	d.wg.Add(1)
	defer d.wg.Done()
	select {
	case <-d.ctx.Done():
		return nil
	default:
		IsActive := d.CheckActive(t)
		d.mux.Lock()
		defer d.mux.Unlock()
		IsExist := CheckExist(t, d.Active, key, d.LogPath)
		switch IsExist {
		case true:
			path := fmt.Sprint("taskdatabase/", t.Format("2006-01-02 15:04:05"), ".db")
			db, err := bolt.Open(path, 0600, &bolt.Options{
				Timeout: 3 * time.Second,
			})
			if err != nil {
				pkg.Log(nil, err, strconv.Itoa(-2), false)
				return err
			}
			defer func() {
				if IsActive == false {
					db.Close()
				}
			}()
			valueb, _ := json.Marshal(value)
			err = db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("tasks"))
				erri := b.Put([]byte(key), valueb)
				if erri != nil {
					pkg.Log(nil, err, strconv.Itoa(-2), false)
					return erri
				}
				return nil
			})
			if err != nil {
				pkg.Log(nil, err, strconv.Itoa(-2), false)
				return err
			}
		case false:
			return errors.New("training task not found")
		}
		return nil
	}
}

func (d *DefaultTaskDb) View(key string, t time.Time) (error, string) {
	d.wg.Add(1)
	defer d.wg.Done()
	select {
	case <-d.ctx.Done():
		return nil, ""
	default:
		var str string
		IsActive := d.CheckActive(t)
		d.mux.RLock()
		defer d.mux.RUnlock()
		IsExist := CheckExist(t, d.Active, key, d.LogPath)
		switch IsExist {
		case true:
			path := fmt.Sprint("taskdatabase/", t.Format("2006-01-02 15:04:05"), ".db")
			db, err := bolt.Open(path, 0600, &bolt.Options{
				Timeout: 3 * time.Second,
			})
			if err != nil {
				pkg.Log(nil, err, strconv.Itoa(-2), false)
				return err, ""
			}
			defer func() {
				if IsActive == false {
					db.Close()
				}
			}()
			err = db.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("tasks"))
				ss := strings.Split(key, "_")
				i, _ := strconv.Atoi(ss[1])
				if i > 0 {
					data := b.Get([]byte(key))
					str = d.PrintData(data)
				} else {
					cursor := b.Cursor()
					for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
						d.PrintData(v)
						fmt.Print("\n")
					}
				}
				return nil
			})
		case false:
			return errors.New("training task not found"), ""
		}
		return nil, str
	}
}

func (d *DefaultTaskDb) Close() error {
	file, err := os.OpenFile("/active.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		pkg.Log(nil, err, strconv.Itoa(-2), false)
		return err
	}
	defer file.Close()
	d.wg.Wait()
	for _, active := range d.Active {
		file.WriteString(pkg.GetTimeString(active.Path()) + "\n")
		err := active.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
