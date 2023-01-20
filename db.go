package main

import (
	"database/sql"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"math"
)

type Database struct {
	db *sql.DB
}

func NewDatabase() (Database, error) {
	db, err := sql.Open("sqlite3", "./instances.db")
	if err != nil {
		return Database{}, errors.Wrap(err, "could not open database")
	}
	_, err = db.Exec("create table if not exists instances (id integer not null primary key, ind integer, name text, processed boolean);" +
		"create table if not exists gitlabs (id integer not null primary key, instance_id integer, allow_signup boolean, email text, pass text, foreign key(instance_id) references instances(id));")
	if err != nil {
		return Database{}, errors.Wrap(err, "could not create table")
	}
	_, err = db.Exec("create unique index if not exists ind_idx on instances (ind);")
	if err != nil {
		return Database{}, errors.Wrap(err, "could not create index")
	}
	_, err = db.Exec("pragma synchronous = NORMAL")
	if err != nil {
		return Database{}, errors.Wrap(err, "could not set synchronous=normal")
	}
	_, err = db.Exec("pragma journal_mode=wal")
	if err != nil {
		return Database{}, errors.Wrap(err, "could not enable wal")
	}
	return Database{db}, nil
}

func (d *Database) Close() {
	err := d.db.Close()
	if err != nil {
		log.Error().Err(err).Msg("error closing database")
	}
}

func (d *Database) IndexRange() (int64, int64, error) {
	res, err := d.db.Query("select count(*) from instances")
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not count rows")
	}
	res.Next()
	var rows int64
	err = res.Scan(&rows)
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not scan row")
	}
	if rows == 0 {
		return math.MaxInt64 / 2, math.MaxInt64 / 2, nil
	}

	res, err = d.db.Query("select max(ind), min(ind) from instances")
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not get index range")
	}
	res.Next()
	var max, min int64
	err = res.Scan(&max, &min)
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not scan row")
	}
	err = res.Close()
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not close results")
	}
	return min, max, nil
}

type UnprocessedInstance struct {
	id   int
	name string
}

func (d *Database) GetUnprocessedPotentialGitLabs() ([]UnprocessedInstance, error) {
	res, err := d.db.Query("select id, name from instances where processed = false and name like 'gitlab.%'")
	if err != nil {
		return nil, err
	}
	instances := make([]UnprocessedInstance, 0)

	for res.Next() {
		instance := UnprocessedInstance{}
		err = res.Scan(&instance.id, &instance.name)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

func (d *Database) AddGitLab(instanceID int, allowSignup bool, email, pass string) error {
	var tx *sql.Tx
	tx, err := d.db.Begin()
	if err != nil {
		return errors.Wrap(err, "could not begin transaction")
	}
	stmt, err := tx.Prepare("insert into gitlabs(instance_id, allow_signup, email, pass) values(?, ?, ?, ?); update instances set processed = true where id = ?")
	if err != nil {
		return errors.Wrap(err, "could not prepares statement")
	}
	_, err = stmt.Exec(instanceID, allowSignup, email, pass)
	if err != nil {
		return errors.Wrap(err, "could not execute statement")
	}
	stmt, err = tx.Prepare("update instances set processed = true where id = ?")
	if err != nil {
		return errors.Wrap(err, "could not prepares statement")
	}
	_, err = stmt.Exec(instanceID)
	if err != nil {
		return errors.Wrap(err, "could not execute statement")
	}
	return tx.Commit()
}

func (d *Database) SetProcessed(instanceID int) error {
	stmt, err := d.db.Prepare("update instances set processed = true where id = ?")
	if err != nil {
		return errors.Wrap(err, "could not prepares statement")
	}
	_, err = stmt.Exec(instanceID)
	if err != nil {
		return errors.Wrap(err, "could not execute statement")
	}
	return err
}

func (d *Database) StoreCertificates(certs []Certificate) {
	var tx *sql.Tx
	tx, err := d.db.Begin()
	if err != nil {
		log.Fatal().Err(err).Msg("could not begin transaction")
	}
	var stmt *sql.Stmt
	stmt, err = tx.Prepare("insert into instances(ind, name, processed) values(?, ?, false)")
	if err != nil {
		log.Fatal().Err(err).Msg("could not prepare statement")
	}
	for _, cert := range certs {
		for _, subject := range cert.Subjects {
			_, err = stmt.Exec(cert.Index, subject)
			if err != nil {
				log.Fatal().Err(err).Msg("could not execute statement")
			}
		}

	}
	err = tx.Commit()
	if err != nil {
		log.Fatal().Err(err).Msg("could not commit transaction")
	}
}
