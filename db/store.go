package db

import (
	"github.com/dgraph-io/badger"
	"github.com/sirupsen/logrus"
)

/*
	Errorf(string, ...interface{})
	Warningf(string, ...interface{})
	Infof(string, ...interface{})
	Debugf(string, ...interface{})
*/

// logging adapter
type badgerLogger struct {
	logger *logrus.Entry
}

func (l *badgerLogger) Errorf(s string, d ...interface{}) {
	l.logger.Errorf(s, d...)
}
func (l *badgerLogger) Warningf(s string, d ...interface{}) {
	l.logger.Warningf(s, d...)
}
func (l *badgerLogger) Infof(s string, d ...interface{}) {
	l.logger.Infof(s, d...)
}
func (l *badgerLogger) Debugf(s string, d ...interface{}) {
	l.logger.Debugf(s, d...)
}

func NewBadgerLoggingAdapter(logger *logrus.Entry) badger.Logger {
	return &badgerLogger{logger: logger}
}

type DBStore struct {
	db *badger.DB
}

func NewDBStore(db *badger.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) ReadTransact(fn func(q ReadQuery) (interface{}, error)) (interface{}, error) {
	var val interface{}
	err := s.db.View(func(tx *badger.Txn) error {
		q := &DBReadQuery{tx: tx}
		v, err := fn(q)
		val = v
		return err

	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (s *DBStore) Transact(fn func(q Query) (interface{}, error)) (interface{}, error) {
	var val interface{}
	err := s.db.Update(func(tx *badger.Txn) error {
		q := &DBQuery{
			&DBReadQuery{tx: tx},
		}
		v, err := fn(q)
		val = v
		return err
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}
