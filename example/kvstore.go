package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/Mathew-Estafanous/raft"
	"net/http"
	"strings"
)

type kvStore struct {
	r    *raft.Raft
	data map[string]string
}

func NewStore() *kvStore {
	return &kvStore{
		data: make(map[string]string),
	}
}

func (k *kvStore) Apply(data []byte) error {
	dataConv := strings.Split(string(data), " ")
	if len(dataConv) != 2 {
		return errors.New("invalid set command")
	}

	k.data[dataConv[0]] = dataConv[1]
	return nil
}

func (k *kvStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	paths := strings.Split(r.URL.Path, "/")
	w.Header().Set("Content-Type", "text/plain")
	switch r.Method {
	case http.MethodGet:
		var b bytes.Buffer
		for k, v := range k.data {
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
		t := k.r.Apply([]byte(cmd))

		if err := t.Error(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
	}
}
