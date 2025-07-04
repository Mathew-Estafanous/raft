package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Mathew-Estafanous/raft"
)

type KvStore struct {
	r    *raft.Raft
	data map[string]string
}

func NewStore() *KvStore {
	return &KvStore{
		data: make(map[string]string),
	}
}

func (k *KvStore) Apply(data []byte) error {
	dataConv := strings.Split(string(data), " ")
	if len(dataConv) != 2 {
		return errors.New("invalid set command")
	}

	k.data[dataConv[0]] = dataConv[1]
	return nil
}

func (k *KvStore) Snapshot() ([]byte, error) {
	var buf bytes.Buffer
	en := gob.NewEncoder(&buf)
	if err := en.Encode(k.data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (k *KvStore) Restore(cmd []byte) error {
	dec := gob.NewDecoder(bytes.NewBuffer(cmd))
	state := make(map[string]string)
	if err := dec.Decode(&state); err != nil {
		return err
	}
	k.data = state
	return nil
}

type kvHandler struct {
	s *KvStore
}

func (k *kvHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	paths := strings.Split(r.URL.Path, "/")
	w.Header().Set("Content-Type", "text/plain")
	switch r.Method {
	case http.MethodGet:
		var b bytes.Buffer
		for k, v := range k.s.data {
			if _, err := fmt.Fprintf(&b, "%v=\"%v\"\n", k, v); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		w.Write(b.Bytes())
	case http.MethodPost:
		if len(paths) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		cmd := strings.Replace(paths[1], "=", " ", 1)
		t := k.s.r.Apply([]byte(cmd))

		if err := t.Error(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
	}
}
