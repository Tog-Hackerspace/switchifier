// Main package for Tog Switchifier. See accompanying README.md file.

// Copyright (c) 2016, Serge Bazanski <s@bazanski.pl>
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice,
// this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation
// and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
)

type switchApp struct {
	db         *sql.DB
	secret     string
	lastUpdate int64
}

func newSwitchApp(db *sql.DB) switchApp {
	return switchApp{
		db:     db,
		secret: secret,
	}
}

func (s *switchApp) RunSchemaUpdate() error {
	sqlStmt := `
CREATE TABLE IF NOT EXISTS switch_state_change(
	timestamp INTEGER NOT NULL PRIMARY KEY,
	interval INTEGER NOT NULL,
	state BOOLEAN NOT NULL
);`
	_, err := s.db.Exec(sqlStmt)
	return err
}

func (s *switchApp) UpdateState(state bool) error {
	defer func() {
		s.lastUpdate = time.Now().UnixNano()
	}()
	sqlStmt := `
SELECT state FROM switch_state_change
	ORDER BY timestamp DESC
	LIMIT 1;`
	res, err := s.db.Query(sqlStmt)
	if err != nil {
		return err
	}

	shouldStore := false
	if !res.Next() {
		// Always store a state if the database is empty
		shouldStore = true
	} else {
		// Otherwise store if there was a state change
		var lastState bool
		if err = res.Scan(&lastState); err != nil {
			res.Close()
			return err
		}
		if lastState != state {
			shouldStore = true
		}
	}
	res.Close()
	if !shouldStore {
		return nil
	}

	glog.Info("Storing state change in database...")
	timestamp := time.Now().UnixNano()
	interval := int64(0)
	if s.lastUpdate != 0 {
		interval = timestamp - s.lastUpdate
	}
	sqlStmt = `INSERT INTO switch_state_change(timestamp, interval, state) VALUES (?, ?, ?)`
	_, err = s.db.Exec(sqlStmt, timestamp, interval, state)
	return err
}

func (s *switchApp) HandleAPIUpdate(w http.ResponseWriter, r *http.Request) {
	glog.Infof("%s: %s %s", r.RemoteAddr, r.Method, r.URL.String())
	if r.Method != "POST" {
		w.WriteHeader(403)
		return
	}
	// Yes, this should be a constant-time comparison.
	if secret := r.FormValue("secret"); secret != s.secret {
		w.WriteHeader(403)
		return
	}

	newValueStr := strings.ToLower(r.FormValue("value"))
	if newValueStr == "" {
		w.WriteHeader(400)
		return
	}
	var newValue bool
	if strings.HasPrefix(newValueStr, "t") || newValueStr == "1" {
		newValue = true
	}

	glog.Infof("Switch state: %v.", newValue)
	err := s.UpdateState(newValue)
	if err != nil {
		w.WriteHeader(500)
		glog.Errorf("Error in handler: %v", err)
		return
	}
	w.WriteHeader(200)
}

type apiGetResponse struct {
	Okay  bool   `json:"okay"`
	Error string `json:"error"`
	Data  struct {
		Open          bool  `json:"open"`
		Since         int64 `json:"since"`
		LastKeepalive int64 `json:"lastKeepalive"`
	} `json:"data"`
}

func (s *switchApp) HandleAPIGet(w http.ResponseWriter, r *http.Request) {
	resp := apiGetResponse{}
	defer func() {
		if resp.Okay {
			resp.Data.LastKeepalive = s.lastUpdate
		}
		respJson, err := json.Marshal(resp)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(`{ okay: false, error: "Internal server error." }`))
			glog.Errorf("Error in handler: %v", err)
		}
		w.Write(respJson)
	}()

	sqlStmt := `SELECT timestamp, state FROM switch_state_change
ORDER BY timestamp DESC
LIMIT 1`
	data, err := s.db.Query(sqlStmt)
	defer data.Close()
	if err != nil {
		resp.Okay = false
		resp.Error = "Database error."
		return
	}
	if !data.Next() {
		resp.Okay = false
		resp.Error = "No switch state yet."
		return
	}
	var timestamp int64
	var state bool
	if data.Scan(&timestamp, &state) != nil {
		resp.Okay = false
		resp.Error = "Database error."
		return
	}
	resp.Okay = true
	resp.Data.Open = state
	resp.Data.Since = timestamp
}

var (
	dbPath      string
	bindAddress string
	secret      string
	secret_path string
)

func main() {
	flag.StringVar(&dbPath, "db_path", "./switchifier.db", "Path to the SQlite3 database.")
	flag.StringVar(&bindAddress, "bind_address", ":8080", "Address to bind HTTP server to.")
	flag.StringVar(&secret, "secret", "changeme", "Secret for state updates.")
	flag.StringVar(&secret_path, "secret_path", "", "File with secret for state updates.")
	flag.Parse()
	if dbPath == "" {
		glog.Exit("Please provide a database path.")
	}
	if secret_path != "" {
		file, err := os.Open(secret_path)
		if err != nil {
			glog.Exit(err)
		}
		secretData, err := ioutil.ReadAll(file)
		if err != nil {
			glog.Exit(err)
		}
		secretParts := strings.Split(string(secretData), "\n")
		secret = secretParts[0]
	}
	glog.Infof("Starting switchifier...")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		glog.Exit(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	switchApp := newSwitchApp(db)
	if err = switchApp.RunSchemaUpdate(); err != nil {
		glog.Exitf("Could not run schema update: %v", err)
	}
	glog.Info("Schema updates applied.")

	http.HandleFunc("/api/1/switchifier/update", switchApp.HandleAPIUpdate)
	http.HandleFunc("/api/1/switchifier/status", switchApp.HandleAPIGet)

	glog.Infof("Listening on %s.", bindAddress)
	glog.Exit(http.ListenAndServe(bindAddress, nil))
}
