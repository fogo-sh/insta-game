package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var ErrNotFound = errors.New("instance not found")
var ErrConflict = errors.New("instance already exists with different settings")
var instancesBucket = []byte("instances-v1")

type Store struct{ db *bolt.DB }

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error { _, err := tx.CreateBucketIfNotExists(instancesBucket); return err })
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func encode(i Instance) ([]byte, error) {
	return json.Marshal(record{Instance: i, StartRequestID: i.StartRequestID, AgentToken: i.AgentToken, RconPassword: i.RconPassword})
}
func decode(data []byte) (Instance, error) {
	if data == nil {
		return Instance{}, ErrNotFound
	}
	var r record
	if err := json.Unmarshal(data, &r); err != nil {
		return Instance{}, err
	}
	r.Instance.StartRequestID = r.StartRequestID
	r.Instance.AgentToken = r.AgentToken
	r.Instance.RconPassword = r.RconPassword
	return r.Instance, nil
}
func (s *Store) Get(id string) (Instance, error) {
	var i Instance
	err := s.db.View(func(tx *bolt.Tx) error {
		var err error
		i, err = decode(tx.Bucket(instancesBucket).Get([]byte(id)))
		return err
	})
	return i, err
}
func (s *Store) List() ([]Instance, error) {
	list := []Instance{}
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(instancesBucket).ForEach(func(_, data []byte) error {
			i, err := decode(data)
			if err != nil {
				return err
			}
			list = append(list, i)
			return nil
		})
	})
	return list, err
}
func (s *Store) Create(candidate Instance) (Instance, error) {
	stored := candidate
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(instancesBucket)
		if data := bucket.Get([]byte(candidate.ID)); data != nil {
			existing, err := decode(data)
			if err != nil {
				return err
			}
			if existing.Provider != candidate.Provider || existing.Game.ID != candidate.Game.ID || existing.Name != candidate.Name || existing.Config != candidate.Config || existing.IdleMinutes != candidate.IdleMinutes || existing.MaxMinutes != candidate.MaxMinutes {
				return ErrConflict
			}
			stored = existing
			return nil
		}
		data, err := encode(candidate)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(candidate.ID), data)
	})
	return stored, err
}

func (s *Store) Update(id string, fn func(*Instance) error) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(instancesBucket)
		i, err := decode(bucket.Get([]byte(id)))
		if err != nil {
			return err
		}
		if err := fn(&i); err != nil {
			return err
		}
		data, err := encode(i)
		if err != nil {
			return fmt.Errorf("encode instance: %w", err)
		}
		return bucket.Put([]byte(id), data)
	})
}
